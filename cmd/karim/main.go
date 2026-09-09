package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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
