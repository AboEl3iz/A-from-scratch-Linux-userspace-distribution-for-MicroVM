package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"karim-microvm-os/internal/netd"
	"karim-microvm-os/internal/stored"
	"karim-microvm-os/internal/svcd"
	"karim-microvm-os/internal/system"
	"karim-microvm-os/internal/vsockd"
)

const defaultServiceDir = "/etc/karim/services"

type supervisorBridge struct {
	managedServices map[string]*svcd.ManagedService
	mu              sync.RWMutex
}

func (sb *supervisorBridge) ListServices() []*vsockd.ServiceInfo {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	var list []*vsockd.ServiceInfo
	for _, ms := range sb.managedServices {
		pid := 0
		if ms.Cmd != nil && ms.Cmd.Process != nil {
			pid = ms.Cmd.Process.Pid
		}
		list = append(list, &vsockd.ServiceInfo{
			Name:           ms.Spec.Name,
			State:          ms.State.String(),
			PID:            pid,
			Exec:           ms.Spec.Exec,
			MemoryLimit:    ms.Spec.MemoryLimit,
			CPUQuota:       ms.Spec.CPUQuota,
			Restart:        ms.Spec.Restart,
			SeccompProfile: ms.Spec.SeccompProfile,
			After:          ms.Spec.After,
		})
	}
	return list
}

func (sb *supervisorBridge) StartService(name string) error {
	sb.mu.RLock()
	ms, ok := sb.managedServices[name]
	sb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("service %s not found", name)
	}
	return ms.Start()
}

func (sb *supervisorBridge) StopService(name string) error {
	sb.mu.RLock()
	ms, ok := sb.managedServices[name]
	sb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("service %s not found", name)
	}
	return ms.Stop()
}

func (sb *supervisorBridge) GetServiceLogs(name string) (string, error) {
	sb.mu.RLock()
	ms, ok := sb.managedServices[name]
	sb.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("service %s not found", name)
	}
	return fmt.Sprintf("[karim-svcd | %s] Active service status: %s\n", name, ms.State.String()), nil
}

func (sb *supervisorBridge) Quiesce() error {
	fmt.Println("[karim-svcd] Guest snapshot quiesce requested: issuing syscall.Sync()...")
	syscall.Sync()
	return nil
}

func (sb *supervisorBridge) Unquiesce() error {
	fmt.Println("[karim-svcd] Guest snapshot thaw requested: resuming normal process execution.")
	return nil
}

func main() {
	serviceDirFlag := flag.String("config-dir", defaultServiceDir, "Directory containing TOML service definitions")
	enableNetFlag := flag.Bool("enable-net", true, "Automatically configure virtio-net interface via netlink")
	enableOverlayFlag := flag.Bool("enable-overlay", true, "Mount writable tmpfs OverlayFS over rootfs")
	enableVsockdFlag := flag.Bool("enable-vsockd", true, "Enable vsockd control plane RPC server")
	flag.Parse()

	fmt.Println("[karim-svcd] Karim MicroVM Supervisor (karim-svcd) starting...")

	if sysStat, err := system.GetSystemStatus(); err == nil {
		fmt.Printf("[karim-svcd] System status verified: entropy=%d bits, rtc=%v, debug=%v\n",
			sysStat.EntropyAvail, sysStat.RTCSynced, sysStat.DebugMode)
	}

	// 1. Storage Overlay Setup (Phase 2 Component)
	if *enableOverlayFlag {
		fmt.Println("[karim-svcd] Initializing OverlayFS storage subsystem...")
		overlayCfg, err := stored.PrepareAndMountRootOverlay("/mnt/lower", "/run/karim/overlay", "/")
		if err != nil {
			fmt.Printf("[karim-svcd] Storage overlay notice/warning: %v\n", err)
		} else {
			fmt.Printf("[karim-svcd] OverlayFS mounted successfully: lower=%s -> target=%s\n", overlayCfg.LowerDir, overlayCfg.TargetDir)
		}
	}

	// 2. Network Interface Bringup (Phase 2 Component)
	if *enableNetFlag {
		fmt.Println("[karim-svcd] Initializing network subsystem (netd)...")
		netCfg := netd.DefaultConfig("eth0")
		if err := netd.AutoConfigure(netCfg); err != nil {
			fmt.Printf("[karim-svcd] Network configuration notice: %v\n", err)
		}
	}

	// 3. Initialize cgroup manager
	cgm, err := svcd.NewCGroupManager()
	if err != nil {
		fmt.Printf("[karim-svcd] Warning: failed to initialize cgroup manager: %v\n", err)
	}

	// 4. Load service configuration files
	fmt.Printf("[karim-svcd] Scanning service configurations in %s...\n", *serviceDirFlag)
	specs, err := svcd.LoadServiceDir(*serviceDirFlag)
	if err != nil {
		fmt.Printf("[karim-svcd] Error loading service configurations: %v\n", err)
	}

	bridge := &supervisorBridge{
		managedServices: make(map[string]*svcd.ManagedService),
	}

	if len(specs) == 0 {
		fmt.Println("[karim-svcd] No service definitions found. Running empty supervisor loop.")
	} else {
		fmt.Printf("[karim-svcd] Loaded %d service specification(s).\n", len(specs))

		// 5. Resolve DAG topological startup order
		graph := svcd.NewDependencyGraph(specs)
		orderedSpecs, err := graph.ResolveTopologicalSort()
		if err != nil {
			fmt.Printf("[karim-svcd] ERROR resolving service dependency graph: %v\n", err)
			os.Exit(1)
		}

		// 6. Start managed services in topological order
		for _, spec := range orderedSpecs {
			fmt.Printf("[karim-svcd] Starting service %s (depends on: %v)...\n", spec.Name, spec.After)
			ms := svcd.NewManagedService(spec, cgm)
			bridge.managedServices[spec.Name] = ms
			if err := ms.Start(); err != nil {
				fmt.Printf("[karim-svcd] ERROR starting service %s: %v\n", spec.Name, err)
			}
			time.Sleep(100 * time.Millisecond) // Stagger start slightly
		}
	}

	// 7. Start vsockd Control Plane RPC Server (Phase 4 Component)
	if *enableVsockdFlag {
		fmt.Println("[karim-svcd] Initializing vsockd host-guest control plane RPC server...")
		rpcServer := vsockd.NewServer(bridge)

		socketPath := "/run/karim/vsock.sock"
		_ = os.MkdirAll(filepath.Dir(socketPath), 0755)

		if vsockd.IsVSockSupported() {
			l, err := vsockd.ListenVSock(vsockd.DefaultVSockPort)
			if err != nil {
				fmt.Printf("[karim-svcd] Notice: ListenVSock failed on port %d: %v\n", vsockd.DefaultVSockPort, err)
			} else {
				fmt.Printf("[karim-svcd] Host-Guest Control Plane active on AF_VSOCK (Port %d)\n", vsockd.DefaultVSockPort)
				go func(lis net.Listener) {
					_ = rpcServer.Serve(lis)
				}(l)
			}
		} else {
			fmt.Println("[karim-svcd] Notice: AF_VSOCK not supported in guest kernel (CONFIG_VIRTIO_VSOCK=y required).")
		}

		if l, err := vsockd.ListenUnix(socketPath); err == nil {
			fmt.Printf("[karim-svcd] Control Plane Unix fallback active on %s\n", socketPath)
			go func(lis net.Listener) {
				_ = rpcServer.Serve(lis)
			}(l)
		}
	}

	// 8. Signal handling loop
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGCHLD)

	fmt.Println("[karim-svcd] Supervisor handoff complete. Entering main monitoring loop.")

	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			fmt.Printf("[karim-svcd] Received termination signal (%v). Shutdown sequence initiated...\n", sig)
			bridge.mu.RLock()
			for name, ms := range bridge.managedServices {
				fmt.Printf("[karim-svcd] Stopping service %s...\n", name)
				_ = ms.Stop()
			}
			bridge.mu.RUnlock()
			os.Exit(0)

		case syscall.SIGCHLD:
			// Asynchronous zombie reaper loop for orphan child processes
			for {
				var wstatus syscall.WaitStatus
				pid, err := syscall.Wait4(-1, &wstatus, syscall.WNOHANG, nil)
				if err != nil || pid <= 0 {
					break
				}
				// Check if PID belongs to a managed service to avoid false 'orphan' log message
				isManaged := false
				bridge.mu.RLock()
				for _, ms := range bridge.managedServices {
					if ms.Cmd != nil && ms.Cmd.Process != nil && ms.Cmd.Process.Pid == pid {
						isManaged = true
						break
					}
				}
				bridge.mu.RUnlock()

				if !isManaged {
					fmt.Printf("[karim-svcd] Reaped orphan zombie process (PID %d)\n", pid)
				}
			}

		}
	}
}
