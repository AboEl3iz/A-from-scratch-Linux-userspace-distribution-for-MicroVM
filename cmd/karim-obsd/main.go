package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"karim-microvm-os/internal/obsd"
	"karim-microvm-os/internal/vsockd"
)

type obsdServer struct {
	mgr *obsd.Manager
	mu  sync.Mutex
}

func newObsdServer(mgr *obsd.Manager) *obsdServer {
	return &obsdServer{mgr: mgr}
}

func (s *obsdServer) handleConn(conn net.Conn) {
	defer conn.Close()

	frame, err := vsockd.ReadFrame(conn)
	if err != nil {
		return
	}

	var req vsockd.RPCRequest
	if err := vsockd.DecodeJSON(frame, &req); err != nil {
		resp, _ := vsockd.EncodeJSON(vsockd.RPCResponse{
			ID:      req.ID,
			Success: false,
			Error:   fmt.Sprintf("invalid json request: %v", err),
		})
		_ = vsockd.WriteFrame(conn, resp)
		return
	}

	switch req.Command {
	case "ping":
		resp, _ := vsockd.EncodeJSON(vsockd.RPCResponse{
			ID:      req.ID,
			Success: true,
			Data:    "PONG from karim-obsd",
		})
		_ = vsockd.WriteFrame(conn, resp)

	case "get_metrics", "get_obsd", "obsd":
		runq, _ := s.mgr.GetRunQueueLatency()
		bio, _ := s.mgr.GetBioLatency()
		data := map[string]any{
			"is_loaded":     s.mgr.IsLoaded(),
			"fallback_mode": s.mgr.IsFallback(),
			"runq_latency":  runq,
			"bio_latency":   bio,
			"prom_metrics":  s.mgr.ExportPrometheusMetrics(),
		}
		resp, _ := vsockd.EncodeJSON(vsockd.RPCResponse{
			ID:      req.ID,
			Success: true,
			Data:    data,
		})
		_ = vsockd.WriteFrame(conn, resp)

	case "stream_exec", "trace":
		resp, _ := vsockd.EncodeJSON(vsockd.RPCResponse{
			ID:      req.ID,
			Success: true,
			Data:    "STREAM_START",
		})
		if err := vsockd.WriteFrame(conn, resp); err != nil {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		eventChan := make(chan obsd.ExecEvent, 20)
		go func() {
			_ = s.mgr.StreamExecEvents(ctx, eventChan)
		}()

		for {
			select {
			case ev, ok := <-eventChan:
				if !ok {
					return
				}
				raw, _ := json.Marshal(ev)
				if err := vsockd.WriteFrame(conn, raw); err != nil {
					return
				}
			}
		}

	default:
		resp, _ := vsockd.EncodeJSON(vsockd.RPCResponse{
			ID:      req.ID,
			Success: false,
			Error:   fmt.Sprintf("unknown command %q", req.Command),
		})
		_ = vsockd.WriteFrame(conn, resp)
	}
}

func main() {
	socketFlag := flag.String("socket", "/run/karim/obsd.sock", "Unix Domain Socket path for obsd RPCs")
	promPortFlag := flag.Int("prom-port", 9100, "Port for HTTP Prometheus metrics endpoint")

	flag.Parse()

	fmt.Println("[karim-obsd] Guest Observability Daemon starting...")

	mgr := obsd.NewManager()
	if err := mgr.LoadProbes(); err != nil {
		fmt.Printf("[karim-obsd] Warning during eBPF probe load: %v\n", err)
	}
	defer mgr.Close()

	// Ensure run socket directory exists
	_ = os.MkdirAll(filepath.Dir(*socketFlag), 0755)
	_ = os.Remove(*socketFlag)

	listener, err := net.Listen("unix", *socketFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[karim-obsd] Failed to listen on Unix socket %s: %v\n", *socketFlag, err)
		os.Exit(1)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(*socketFlag)
	}()

	server := newObsdServer(mgr)

	// Start Unix socket listener loop
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.handleConn(conn)
		}
	}()
	fmt.Printf("[karim-obsd] Listening for telemetry RPCs on %s\n", *socketFlag)

	// Optional HTTP Prometheus exporter
	if *promPortFlag > 0 {
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			_, _ = w.Write([]byte(mgr.ExportPrometheusMetrics()))
		})
		go func() {
			addr := fmt.Sprintf(":%d", *promPortFlag)
			fmt.Printf("[karim-obsd] Serving Prometheus metrics on http://0.0.0.0%s/metrics\n", addr)
			_ = http.ListenAndServe(addr, nil)
		}()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	fmt.Printf("[karim-obsd] Shutdown signal received (%v). Exiting...\n", sig)
}
