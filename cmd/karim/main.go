package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"karim-microvm-os/internal/obsd"
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
	fmt.Println(asciiLogo)
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
