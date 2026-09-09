package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"karim-microvm-os/internal/netd"
	"karim-microvm-os/internal/stored"
	"karim-microvm-os/internal/svcd"
)

const defaultServiceDir = "/etc/karim/services"

func main() {
	serviceDirFlag := flag.String("config-dir", defaultServiceDir, "Directory containing TOML service definitions")
	enableNetFlag := flag.Bool("enable-net", true, "Automatically configure virtio-net interface via netlink")
	enableOverlayFlag := flag.Bool("enable-overlay", false, "Mount writable tmpfs OverlayFS over rootfs")
	flag.Parse()

	fmt.Println("[karim-svcd] Karim MicroVM Supervisor (karim-svcd) starting...")

	// 1. Storage Overlay Setup (Phase 2 Component)
	if *enableOverlayFlag {
		fmt.Println("[karim-svcd] Initializing OverlayFS storage subsystem...")
		overlayCfg, err := stored.PrepareAndMountRootOverlay("/mnt/lower", "/run/karim/overlay", "/mnt/root")
		if err != nil {
			fmt.Printf("[karim-svcd] Storage overlay notice/warning: %v\n", err)
		} else {
			fmt.Printf("[karim-svcd] OverlayFS mounted successfully: %+v\n", overlayCfg)
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
		var managedServices []*svcd.ManagedService
		for _, spec := range orderedSpecs {
			fmt.Printf("[karim-svcd] Starting service %s (depends on: %v)...\n", spec.Name, spec.After)
			ms := svcd.NewManagedService(spec, cgm)
			if err := ms.Start(); err != nil {
				fmt.Printf("[karim-svcd] ERROR starting service %s: %v\n", spec.Name, err)
			}
			managedServices = append(managedServices, ms)
			time.Sleep(100 * time.Millisecond) // Stagger start slightly
		}
	}

	// 7. Signal handling loop
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGCHLD)

	fmt.Println("[karim-svcd] Supervisor handoff complete. Entering main monitoring loop.")

	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			fmt.Printf("[karim-svcd] Received termination signal (%v). Shutdown sequence initiated...\n", sig)
			os.Exit(0)
		case syscall.SIGCHLD:
			// Signal reaper notification
		}
	}
}

