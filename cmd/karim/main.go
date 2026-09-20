package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"karim-microvm-os/internal/builder"
	"karim-microvm-os/internal/obsd"
	"karim-microvm-os/internal/pkgd"
	"karim-microvm-os/internal/snapshot"
	"karim-microvm-os/internal/system"
	"karim-microvm-os/internal/vsockd"

)

const asciiLogo = `
 _  __          _                          
| |/ /__ _ _ __(_)_ __ ___       ___  ___  
| ' // _  | '__| | '_  _ \     / _ \/ __| 
| . \ (_| | |  | | | | | | |   | (_) \__ \ 
|_|\_\__,_|_|  |_|_| |_| |_|    \___/|___/ 
                                           
  tiny init. real kernel. one job.
`

func printHelp() {
	fmt.Print(asciiLogo)
	fmt.Println("Usage: karim [--target <addr>] <command> [args...]")
	fmt.Println("\nTarget Format:")
	fmt.Println("  vsock://CID:PORT           (e.g., vsock://3:1024 for QEMU microVM guest)")
	fmt.Println("  unix:///path/to/socket     (e.g., unix:///run/karim/vsock.sock)")
	fmt.Println("  tcp://host:port            (e.g., tcp://127.0.0.1:1024)")
	fmt.Println("\nCommands:")
	fmt.Println("  ping                       Check connectivity to guest karim-vsockd")
	fmt.Println("  ps                         List running microVM services and status")
	fmt.Println("  start <service>            Start a declared microVM service")
	fmt.Println("  stop <service>             Gracefully stop a running service")
	fmt.Println("  logs <service>             Display stdout/stderr log output for a service")
	fmt.Println("  metrics                    Fetch microVM performance and memory metrics")
	fmt.Println("  obsd                       Fetch eBPF kernel latency histograms & metrics")
	fmt.Println("  trace                      Stream live traced process executions")
	fmt.Println("  build                      Build hermetic initramfs and SquashFS images")
	fmt.Println("  import <archive.tar>       Inspect or import an OCI/Docker image tarball")
	fmt.Println("  snapshot <subcommand>      Orchestrate microVM QMP state save/restore/list")
	fmt.Println("  system                     Fetch guest hardware entropy, RTC sync & debug status")
	fmt.Println("  help                       Show this help menu")
}

func main() {
	targetFlag := flag.String("target", "vsock://3:1024", "Target control plane address (vsock://3:1024, unix:///path, tcp://host:port)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}

	command := strings.ToLower(args[0])

	switch command {
	case "help", "-h", "--help":
		printHelp()
		return

	case "ping":
		runPing(*targetFlag)

	case "ps", "list":
		runPS(*targetFlag)

	case "start":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: missing service name. Usage: karim start <service>\n")
			os.Exit(1)
		}
		runStart(*targetFlag, args[1])

	case "stop":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: missing service name. Usage: karim stop <service>\n")
			os.Exit(1)
		}
		runStop(*targetFlag, args[1])

	case "logs":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: missing service name. Usage: karim logs <service>\n")
			os.Exit(1)
		}
		runLogs(*targetFlag, args[1])

	case "metrics", "top":
		runMetrics(*targetFlag)

	case "obsd", "ebpf":
		runObsd(*targetFlag)

	case "trace", "execsnoop":
		runTrace(*targetFlag)

	case "build", "image":
		runBuild(args[1:])

	case "import", "layer":
		runLayer(args[1:])

	case "snapshot", "qmp":
		runSnapshot(*targetFlag, args[1:])

	case "system", "status", "hardd":
		runSystem(*targetFlag)


	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q. Run 'karim help' for options.\n", command)
		os.Exit(1)
	}
}

func sendRPC(targetAddr, command, service string, args []string) (*vsockd.RPCResponse, error) {
	conn, err := vsockd.Dial(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to %s: %w", targetAddr, err)
	}
	defer conn.Close()

	req := vsockd.RPCRequest{
		ID:      "req-1",
		Command: command,
		Service: service,
		Args:    args,
	}

	reqBytes, err := vsockd.EncodeJSON(req)
	if err != nil {
		return nil, fmt.Errorf("failed encoding request: %w", err)
	}

	if err := vsockd.WriteFrame(conn, reqBytes); err != nil {
		return nil, fmt.Errorf("failed writing request frame: %w", err)
	}

	respBytes, err := vsockd.ReadFrame(conn)
	if err != nil {
		return nil, fmt.Errorf("failed reading response frame: %w", err)
	}

	var resp vsockd.RPCResponse
	if err := vsockd.DecodeJSON(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed decoding response: %w", err)
	}

	return &resp, nil
}

func runPing(target string) {
	resp, err := sendRPC(target, "ping", "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ping failed: %v\n", err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Ping error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Printf("[karim-cli] Connection to %s successful: %v\n", target, resp.Data)
}

func runPS(target string) {
	resp, err := sendRPC(target, "list_services", "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing services: %v\n", err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}

	raw, _ := json.Marshal(resp.Data)
	var services []vsockd.ServiceInfo
	_ = json.Unmarshal(raw, &services)

	if len(services) == 0 {
		fmt.Println("No active services reported by supervisor.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tSTATE\tPID\tEXEC\tMEMORY\tSECCOMP\tRESTART")
	fmt.Fprintln(w, "-------\t-----\t---\t----\t------\t-------\t-------")

	for _, s := range services {
		memStr := "unlimited"
		if s.MemoryLimit > 0 {
			memStr = fmt.Sprintf("%dMB", s.MemoryLimit/(1024*1024))
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			s.Name, s.State, s.PID, s.Exec, memStr, s.SeccompProfile, s.Restart)
	}
	w.Flush()
}

func runStart(target, service string) {
	resp, err := sendRPC(target, "start_service", service, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting service %s: %v\n", service, err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Printf("[karim-cli] %v\n", resp.Data)
}

func runStop(target, service string) {
	resp, err := sendRPC(target, "stop_service", service, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping service %s: %v\n", service, err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Printf("[karim-cli] %v\n", resp.Data)
}

func runLogs(target, service string) {
	resp, err := sendRPC(target, "get_logs", service, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching logs for %s: %v\n", service, err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Print(resp.Data)
}

func runMetrics(target string) {
	resp, err := sendRPC(target, "get_metrics", "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching metrics: %v\n", err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}

	raw, _ := json.Marshal(resp.Data)
	var metrics vsockd.SystemMetrics
	_ = json.Unmarshal(raw, &metrics)

	fmt.Println("=== Karim MicroVM Guest Performance Metrics ===")
	fmt.Printf("  Guest Uptime:        %.2f seconds\n", metrics.UptimeSeconds)
	fmt.Printf("  Active Goroutines:   %d\n", metrics.NumGoroutine)
	fmt.Printf("  Memory Allocated:    %.2f MB\n", float64(metrics.MemoryAlloc)/(1024*1024))
	fmt.Printf("  Memory System:       %.2f MB\n", float64(metrics.MemorySys)/(1024*1024))
	fmt.Printf("  Managed Services:    %d\n", metrics.NumServices)
}

type HistData struct {
	Name        string     `json:"name"`
	Slots       [20]uint64 `json:"slots"`
	TotalCount  uint64     `json:"total_count"`
	LastUpdated string     `json:"last_updated"`
}

type ObsdResponseData struct {
	IsLoaded     bool     `json:"is_loaded"`
	FallbackMode bool     `json:"fallback_mode"`
	RunqLatency  HistData `json:"runq_latency"`
	BioLatency   HistData `json:"bio_latency"`
	PromMetrics  string   `json:"prom_metrics"`
}

func getSlotRange(idx int) string {
	if idx == 0 {
		return "0 -> 1 us"
	}
	low := 1 << (idx - 1)
	high := 1 << idx
	if idx >= 19 {
		return fmt.Sprintf("%d+ us", low)
	}
	return fmt.Sprintf("%d -> %d us", low, high)
}

func printHistogram(title string, hist HistData) {
	fmt.Printf("\n--- %s ---\n", title)
	if hist.TotalCount == 0 {
		fmt.Println("  (No events recorded in kernel histogram buffer)")
		return
	}

	var maxCount uint64
	for _, c := range hist.Slots {
		if c > maxCount {
			maxCount = c
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "LATENCY RANGE\tCOUNT\tDISTRIBUTION")
	fmt.Fprintln(w, "-------------\t-----\t------------")

	for i, count := range hist.Slots {
		if count == 0 {
			continue
		}
		barLen := 0
		if maxCount > 0 {
			barLen = int((count * 35) / maxCount)
		}
		if barLen == 0 && count > 0 {
			barLen = 1
		}
		bar := strings.Repeat("█", barLen)
		fmt.Fprintf(w, "%s\t%d\t%s\n", getSlotRange(i), count, bar)
	}
	w.Flush()
}

func runObsd(target string) {
	resp, err := sendRPC(target, "obsd", "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching obsd telemetry: %v\n", err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}

	raw, _ := json.Marshal(resp.Data)
	var data ObsdResponseData
	if err := json.Unmarshal(raw, &data); err != nil {
		fmt.Println("=== Karim MicroVM eBPF Telemetry Raw Output ===")
		fmt.Println(string(raw))
		return
	}

	statusStr := "ACTIVE (Kernel CO-RE BTF probes loaded)"
	if data.FallbackMode || !data.IsLoaded {
		statusStr = "ACTIVE (Simulation Telemetry Fallback Mode)"
	}

	fmt.Println("======================================================================")
	fmt.Println("        Karim MicroVM OS — eBPF Observability Engine Telemetry        ")
	fmt.Println("======================================================================")
	fmt.Printf("Probe Status:     %s\n", statusStr)
	fmt.Printf("Kernel Probes:    sched_process_exec (ringbuffer)\n")
	fmt.Printf("                  sched_wakeup / sched_switch (runqlat)\n")
	fmt.Printf("                  block_rq_issue / block_rq_complete (biolatency)\n")

	printHistogram("CPU Scheduler Run-Queue Latency (sched_runq_latency)", data.RunqLatency)
	printHistogram("Block I/O Completion Latency (block_io_latency)", data.BioLatency)
	fmt.Println("\n======================================================================")
}

func runTrace(target string) {
	conn, err := vsockd.Dial(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed connecting to %s: %v\n", target, err)
		os.Exit(1)
	}
	defer conn.Close()

	reqBytes, _ := vsockd.EncodeJSON(vsockd.RPCRequest{ID: "trace-1", Command: "trace"})
	if err := vsockd.WriteFrame(conn, reqBytes); err != nil {
		fmt.Fprintf(os.Stderr, "Failed sending trace request: %v\n", err)
		os.Exit(1)
	}

	respFrame, err := vsockd.ReadFrame(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed reading response frame: %v\n", err)
		os.Exit(1)
	}

	var initResp vsockd.RPCResponse
	if err := vsockd.DecodeJSON(respFrame, &initResp); err == nil && !initResp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", initResp.Error)
		os.Exit(1)
	}

	fmt.Println("======================================================================")
	fmt.Println("        Karim MicroVM OS — Traced Process Execution Stream            ")
	fmt.Println("======================================================================")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TIME\tPID\tPPID\tCOMM\tFILENAME")
	fmt.Fprintln(w, "----\t---\t----\t----\t--------")
	w.Flush()

	for count := 0; count < 8; count++ {
		frame, err := vsockd.ReadFrame(conn)
		if err != nil {
			break
		}

		var ev obsd.ExecEvent
		if err := json.Unmarshal(frame, &ev); err != nil {
			continue
		}

		tsStr := ev.Timestamp.Format("15:04:05.000")
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n", tsStr, ev.PID, ev.PPID, ev.Comm, ev.Filename)
		w.Flush()
	}
	fmt.Println("======================================================================")
}

func runBuild(cmdArgs []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	outDir := fs.String("out-dir", "dist", "Output directory for compiled images")
	initBin := fs.String("init-bin", "build/init", "Path to compiled C static init binary")
	svcdBin := fs.String("svcd-bin", "build/karim-svcd", "Path to compiled Go svcd supervisor binary")
	servicesDir := fs.String("services-dir", "config/services", "Directory containing TOML service definitions")
	layersFile := fs.String("layers-file", "config/layers.toml", "Path to package layers specification file")
	kernelPath := fs.String("kernel", "dist/bzImage", "Path to compiled bzImage kernel")
	epoch := fs.Int64("epoch", 0, "Fixed timestamp epoch for hermetic build reproducibility (SOURCE_DATE_EPOCH)")
	verify := fs.Bool("verify-reproducible", false, "Perform double-build byte-for-byte SHA-256 reproducibility check")

	if err := fs.Parse(cmdArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing build flags: %v\n", err)
		os.Exit(1)
	}

	cfg := builder.BuildConfig{
		InitBinPath:     *initBin,
		SvcdBinPath:     *svcdBin,
		ServicesDir:     *servicesDir,
		LayersFile:      *layersFile,
		KernelPath:      *kernelPath,
		OutputDir:       *outDir,
		SourceDateEpoch: *epoch,
	}

	fmt.Println("======================================================================")
	fmt.Println("      Karim MicroVM OS — Hermetic Reproducible Image Builder          ")
	fmt.Println("======================================================================")
	fmt.Printf("Init Binary:      %s\n", cfg.InitBinPath)
	fmt.Printf("Svcd Binary:      %s\n", cfg.SvcdBinPath)
	fmt.Printf("Services Dir:     %s\n", cfg.ServicesDir)
	fmt.Printf("Output Dir:       %s\n", cfg.OutputDir)
	fmt.Printf("Source Date Epoch: %d (Hermetic Epoch 0)\n", cfg.SourceDateEpoch)

	if *verify {
		fmt.Println("\n==> Initiating Double-Build Hermetic Reproducibility Audit...")
		if err := builder.VerifyReproducibility(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ REPRODUCIBILITY AUDIT FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ REPRODUCIBILITY AUDIT PASSED: 100% Byte-for-Byte Identical Output Images!")
		return
	}

	fmt.Println("\n==> Assembling Hermetic Images...")
	manifest, err := builder.BuildImagePipeline(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("======================================================================")
	fmt.Println("                     Build Completed Successfully!                    ")
	fmt.Println("======================================================================")
	fmt.Printf("Combined Build SHA-256 Hash: %s\n\n", manifest.CombinedBuildHash)
	fmt.Println("Generated Artifacts:")
	for k, a := range manifest.OutputArtifacts {
		fmt.Printf("  - %-18s (%d bytes, SHA-256: %s)\n", k, a.SizeBytes, a.SHA256)
	}
	fmt.Printf("\nManifest saved at: %s/manifest.json\n", cfg.OutputDir)
}

func runLayer(cmdArgs []string) {
	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: karim import <oci-archive.tar | oci:image:tag | docker:image:tag> [layer-name]\n")
		os.Exit(1)
	}

	rawInput := cmdArgs[0]
	archivePath := rawInput
	layerName := "imported-oci"
	if len(cmdArgs) > 1 {
		layerName = cmdArgs[1]
	}

	fmt.Println("======================================================================")
	fmt.Println("       Karim MicroVM OS — OCI Image & Layer Engine (pkgd)             ")
	fmt.Println("======================================================================")

	// If input is not a local file, attempt auto-export via docker save
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		cleanRef := strings.TrimPrefix(rawInput, "docker-archive:")
		cleanRef = strings.TrimPrefix(cleanRef, "oci:")
		cleanRef = strings.TrimPrefix(cleanRef, "docker:")

		if strings.HasSuffix(cleanRef, ".tar") || strings.HasSuffix(cleanRef, ".tgz") || strings.HasSuffix(cleanRef, ".tar.gz") {
			fmt.Fprintf(os.Stderr, "❌ Error: File %q not found in current directory.\n", cleanRef)
			fmt.Fprintf(os.Stderr, "   Make sure the tarball path exists, or specify a container image reference like:\n")
			fmt.Fprintf(os.Stderr, "   ./dist/karim import oci:alpine:latest\n")
			os.Exit(1)
		}

		fmt.Printf("Notice: Local file %q not found. Attempting auto-export via 'docker save %s'...\n", rawInput, cleanRef)

		sanitized := strings.ReplaceAll(strings.ReplaceAll(cleanRef, "/", "_"), ":", "_")
		tmpTar := filepath.Join(os.TempDir(), fmt.Sprintf("karim_docker_%s.tar", sanitized))

		cmd := exec.Command("docker", "save", cleanRef, "-o", tmpTar)
		out, execErr := cmd.CombinedOutput()
		if execErr != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: Failed opening OCI tar file %q: file not found.\n", rawInput)
			fmt.Fprintf(os.Stderr, "   Docker auto-export also failed (%v):\n   %s\n", execErr, string(out))
			fmt.Fprintf(os.Stderr, "Tip: To import a container image, pull it first:\n")
			fmt.Fprintf(os.Stderr, "     docker pull %s\n", cleanRef)
			fmt.Fprintf(os.Stderr, "     ./dist/karim import oci:%s\n", cleanRef)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully exported Docker image %s to %s\n", cleanRef, tmpTar)
		archivePath = tmpTar
		if len(cmdArgs) == 1 {
			layerName = sanitized
		}
	}

	fmt.Printf("Inspecting OCI archive: %s...\n", archivePath)

	info, err := pkgd.InspectOCIArchive(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inspecting OCI archive: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Repo Tags:    %v\n", info.RepoTags)
	fmt.Printf("Layer Count:  %d layers detected\n", len(info.LayerPaths))
	fmt.Println("Layer Blobs:")
	for i, l := range info.LayerPaths {
		fmt.Printf("  [%d] %s\n", i+1, l)
	}

	// Register layer in config/layers.toml
	cfgFile := "config/layers.toml"
	cfg, _ := pkgd.LoadLayerConfig(cfgFile)

	newSpec := pkgd.LayerSpec{
		Name:      layerName,
		Type:      pkgd.LayerTypeOCIArchive,
		Path:      archivePath,
		TargetDir: "/",
	}

	found := false
	for i, l := range cfg.Layers {
		if l.Name == layerName {
			cfg.Layers[i] = newSpec
			found = true
			break
		}
	}
	if !found {
		cfg.Layers = append(cfg.Layers, newSpec)
	}

	if err := pkgd.SaveLayerConfig(cfg, cfgFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed registering layer in %s: %v\n", cfgFile, err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Successfully registered OCI layer %q in %s!\n", layerName, cfgFile)
}

func runSnapshot(vsockTarget string, cmdArgs []string) {
	if len(cmdArgs) == 0 {
		fmt.Println("======================================================================")
		fmt.Println("    Karim MicroVM OS — QMP Snapshot & Restore Orchestration (Phase 8) ")
		fmt.Println("======================================================================")
		fmt.Println("Usage: karim snapshot <subcommand> [flags]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  save <tag>             Save microVM state to named snapshot (quiesces guest VFS)")
		fmt.Println("  restore <tag>          Restore microVM state from named snapshot")
		fmt.Println("  list                   List all recorded state snapshots")
		fmt.Println("  delete <tag>           Delete a state snapshot from QEMU block storage")
		fmt.Println("  status                 Query QEMU hypervisor CPU execution status")
		fmt.Println("\nFlags:")
		fmt.Println("  --qmp-socket <path>    Path to QMP Unix domain socket (default: /tmp/qmp.sock)")
		fmt.Println("  --pause-vm             Pause CPU execution during savevm (default: true)")
		return
	}

	subCmd := strings.ToLower(cmdArgs[0])
	subArgs := cmdArgs[1:]

	// Separate flags from positional arguments to support flexible flag ordering
	var flagArgs []string
	var positionalArgs []string
	for i := 0; i < len(subArgs); i++ {
		arg := subArgs[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "--qmp-socket" || arg == "-qmp-socket") && i+1 < len(subArgs) {
				i++
				flagArgs = append(flagArgs, subArgs[i])
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	qmpSock := fs.String("qmp-socket", "/tmp/qmp.sock", "Path to QMP Unix socket endpoint")
	pauseVM := fs.Bool("pause-vm", true, "Pause CPU execution during snapshot state dump")
	_ = fs.Parse(append(flagArgs, positionalArgs...))

	// Respect KARIM_QMP_SOCKET environment variable if set
	if envSock := os.Getenv("KARIM_QMP_SOCKET"); envSock != "" {
		*qmpSock = envSock
	}

	orch := snapshot.NewOrchestrator(snapshot.SnapshotConfig{
		QMPSocketPath: *qmpSock,
		VSockTarget:   vsockTarget,
		PauseVM:       *pauseVM,
	})

	fmt.Println("======================================================================")
	fmt.Println("    Karim MicroVM OS — QMP Snapshot & Restore Orchestration (Phase 8) ")
	fmt.Println("======================================================================")

	switch subCmd {
	case "save", "create":
		positional := fs.Args()
		if len(positional) < 1 {
			fmt.Fprintf(os.Stderr, "Error: missing snapshot tag name. Usage: karim snapshot save <tag>\n")
			os.Exit(1)
		}
		tag := positional[0]
		fmt.Printf("Initiating MicroVM snapshot save (tag: %q, QMP: %s)...\n", tag, *qmpSock)

		res, err := orch.SaveSnapshot(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Snapshot save failed: %v\n", err)
			os.Exit(1)
		}

		quiesceStr := "NO (VSOCK un-reachable)"
		if res.Quiesced {
			quiesceStr = "YES (filesystem buffers flushed via syscall.Sync)"
		}

		fmt.Println("\n✅ MicroVM State Snapshot Saved Successfully!")
		fmt.Printf("  Snapshot Tag:    %s\n", res.Tag)
		fmt.Printf("  Guest Quiesced:  %s\n", quiesceStr)
		fmt.Printf("  Elapsed Time:    %v\n", res.Duration)
		if res.Output != "" {
			fmt.Printf("  QMP Output:      %s\n", strings.TrimSpace(res.Output))
		}

	case "restore", "load":
		positional := fs.Args()
		if len(positional) < 1 {
			fmt.Fprintf(os.Stderr, "Error: missing snapshot tag name. Usage: karim snapshot restore <tag>\n")
			os.Exit(1)
		}
		tag := positional[0]
		fmt.Printf("Initiating MicroVM snapshot restore (tag: %q, QMP: %s)...\n", tag, *qmpSock)

		res, err := orch.RestoreSnapshot(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Snapshot restore failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n✅ MicroVM State Snapshot Restored Successfully!")
		fmt.Printf("  Snapshot Tag:    %s\n", res.Tag)
		fmt.Printf("  Elapsed Time:    %v\n", res.Duration)
		if res.Output != "" {
			fmt.Printf("  QMP Output:      %s\n", strings.TrimSpace(res.Output))
		}

	case "list", "ls":
		fmt.Printf("Querying QEMU snapshots via QMP (%s)...\n", *qmpSock)
		snaps, err := orch.ListSnapshots()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed querying snapshots: %v\n", err)
			os.Exit(1)
		}

		if len(snaps) == 0 {
			fmt.Println("No recorded state snapshots found in hypervisor storage.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tTAG\tVM SIZE\tDATE\tVM CLOCK")
		fmt.Fprintln(w, "--\t---\t-------\t----\t--------")
		for _, s := range snaps {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Tag, s.VMSize, s.Date, s.VMClock)
		}
		w.Flush()

	case "delete", "del", "rm":
		positional := fs.Args()
		if len(positional) < 1 {
			fmt.Fprintf(os.Stderr, "Error: missing snapshot tag name. Usage: karim snapshot delete <tag>\n")
			os.Exit(1)
		}
		tag := positional[0]
		fmt.Printf("Deleting snapshot tag %q via QMP (%s)...\n", tag, *qmpSock)

		if err := orch.DeleteSnapshot(tag); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Delete snapshot failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Snapshot %q deleted successfully.\n", tag)

	case "status":
		status, err := orch.QueryStatus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed querying QMP status: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Hypervisor QMP Status: %s (Running: %v, Singlestep: %v)\n",
			status.Status, status.Running, status.Singlestep)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown snapshot subcommand %q. Run 'karim snapshot' for options.\n", subCmd)
		os.Exit(1)
	}
}

func runSystem(target string) {
	resp, err := sendRPC(target, "get_system_status", "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching system status: %v\n", err)
		os.Exit(1)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", resp.Error)
		os.Exit(1)
	}

	raw, _ := json.Marshal(resp.Data)
	var sys system.SystemStatus
	if err := json.Unmarshal(raw, &sys); err != nil {
		fmt.Printf("Raw system status data: %s\n", string(raw))
		return
	}

	entropyStatus := fmt.Sprintf("%d bits (HEALTHY)", sys.EntropyAvail)
	if sys.EntropyAvail < 1000 {
		entropyStatus = fmt.Sprintf("%d bits (LOW)", sys.EntropyAvail)
	}

	rtcStatus := "DISABLED / ABSENT"
	if sys.RTCSynced {
		rtcStatus = "ACTIVE (/dev/rtc0 hardware clock)"
	}

	debugStatus := "DISABLED (Production Mode)"
	if sys.DebugMode {
		debugStatus = "ACTIVE (karim.debug=1 cmdline flag set)"
	}

	fmt.Println("======================================================================")
	fmt.Println("    Karim MicroVM OS — Phase 9 System, Entropy & RTC Hardening Status ")
	fmt.Println("======================================================================")
	fmt.Printf("  Kernel Entropy Pool:  %s\n", entropyStatus)
	fmt.Printf("  Real-Time Clock:      %s\n", rtcStatus)
	fmt.Printf("  Current Guest Time:   %s\n", sys.CurrentTime.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("  Debug Shell Mode:     %s\n", debugStatus)
	fmt.Printf("  Guest Uptime:         %.2f seconds\n", sys.UptimeSeconds)
	if sys.KernelCmdline != "" {
		fmt.Printf("  Kernel Command Line:  %s\n", sys.KernelCmdline)
	}
	fmt.Println("======================================================================")
}



