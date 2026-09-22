package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"karim-microvm-os/internal/svcd"
	"karim-microvm-os/internal/vsockd"
)

type svcdBridge struct {
	serviceDir string
	cgm        *svcd.CGroupManager
	mu         sync.Mutex
	services   map[string]*svcd.ManagedService
}

func newSvcdBridge(serviceDir string) *svcdBridge {
	cgm, _ := svcd.NewCGroupManager()
	b := &svcdBridge{
		serviceDir: serviceDir,
		cgm:        cgm,
		services:   make(map[string]*svcd.ManagedService),
	}
	b.reloadSpecs()
	return b
}

func (b *svcdBridge) reloadSpecs() {
	b.mu.Lock()
	defer b.mu.Unlock()

	specs, err := svcd.LoadServiceDir(b.serviceDir)
	if err != nil || len(specs) == 0 {
		return
	}

	for _, spec := range specs {
		if _, ok := b.services[spec.Name]; !ok {
			b.services[spec.Name] = svcd.NewManagedService(spec, b.cgm)
		} else {
			b.services[spec.Name].Spec = spec
		}
	}
}

func (b *svcdBridge) ListServices() []*vsockd.ServiceInfo {
	b.reloadSpecs()
	b.mu.Lock()
	defer b.mu.Unlock()

	var list []*vsockd.ServiceInfo
	for _, ms := range b.services {
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

func (b *svcdBridge) StartService(name string) error {
	b.mu.Lock()
	ms, ok := b.services[name]
	b.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %q not found", name)
	}
	return ms.Start()
}

func (b *svcdBridge) StopService(name string) error {
	b.mu.Lock()
	ms, ok := b.services[name]
	b.mu.Unlock()

	if !ok {
		return fmt.Errorf("service %q not found", name)
	}
	return ms.Stop()
}

func (b *svcdBridge) GetServiceLogs(name string) (string, error) {
	b.mu.Lock()
	_, ok := b.services[name]
	b.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("service %q not found", name)
	}
	return fmt.Sprintf("[vsockd] Active service telemetry log buffer for %s\n", name), nil
}

func (b *svcdBridge) Quiesce() error {
	fmt.Println("[karim-vsockd] Guest snapshot quiesce requested: flushing filesystem buffers via syscall.Sync()...")
	syscall.Sync()
	return nil
}

func (b *svcdBridge) Unquiesce() error {
	fmt.Println("[karim-vsockd] Guest snapshot thaw requested: microVM state restored and active.")
	return nil
}

// LoadService parses an in-memory TOML service definition and launches it live without a reboot.
func (b *svcdBridge) LoadService(configTOML string) error {
	spec, err := svcd.ParseServiceSpecFromBytes([]byte(configTOML))
	if err != nil {
		return fmt.Errorf("failed to parse service TOML payload: %w", err)
	}

	fmt.Printf("[karim-vsockd] Hot-loading service %q from VSOCK payload...\n", spec.Name)

	ms := svcd.NewManagedService(spec, b.cgm)

	b.mu.Lock()
	if existing, ok := b.services[spec.Name]; ok {
		_ = existing.Stop()
	}
	b.services[spec.Name] = ms
	b.mu.Unlock()

	return ms.Start()
}


func main() {
	portFlag := flag.Uint("port", uint(vsockd.DefaultVSockPort), "AF_VSOCK port to listen on")
	socketFlag := flag.String("socket", "/run/karim/vsock.sock", "Unix Domain Socket path fallback")
	configDirFlag := flag.String("config-dir", "/etc/karim/services", "Service definitions directory")

	flag.Parse()

	fmt.Println("[karim-vsockd] Guest Control Plane Daemon starting...")

	bridge := newSvcdBridge(*configDirFlag)
	server := vsockd.NewServer(bridge)

	// Ensure run directory exists
	_ = os.MkdirAll(filepath.Dir(*socketFlag), 0755)

	// Try starting AF_VSOCK listener
	if vsockd.IsVSockSupported() {
		vsockListener, err := vsockd.ListenVSock(uint32(*portFlag))
		if err != nil {
			fmt.Printf("[karim-vsockd] VSock listener notice: %v\n", err)
		} else {
			fmt.Printf("[karim-vsockd] Listening on AF_VSOCK (CID ANY, Port %d)\n", *portFlag)
			go func() {
				if err := server.Serve(vsockListener); err != nil {
					fmt.Printf("[karim-vsockd] VSock server error: %v\n", err)
				}
			}()
		}
	} else {
		fmt.Println("[karim-vsockd] Notice: AF_VSOCK transport not available on host/guest kernel.")
	}

	// Always start Unix Domain Socket fallback listener
	unixListener, err := vsockd.ListenUnix(*socketFlag)
	if err != nil {
		fmt.Printf("[karim-vsockd] Warning: failed to listen on Unix socket %s: %v\n", *socketFlag, err)
	} else {
		fmt.Printf("[karim-vsockd] Listening on Unix Domain Socket (%s)\n", *socketFlag)
		go func() {
			if err := server.Serve(unixListener); err != nil {
				fmt.Printf("[karim-vsockd] Unix server error: %v\n", err)
			}
		}()
	}

	// Handle graceful shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	fmt.Printf("[karim-vsockd] Shutdown signal received (%v). Exiting...\n", sig)
	_ = server.Close()
	_ = os.Remove(*socketFlag)
}
