package vsockd

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"
)

// ServiceProvider defines the bridge interface between vsockd control server and svcd process manager.
type ServiceProvider interface {
	ListServices() []*ServiceInfo
	StartService(name string) error
	StopService(name string) error
	GetServiceLogs(name string) (string, error)
}

// Server encapsulates the vsockd RPC control plane daemon.
type Server struct {
	provider  ServiceProvider
	startTime time.Time
	mu        sync.RWMutex
	listeners []net.Listener
	closed    bool
}

// NewServer creates a new vsockd RPC server instance.
func NewServer(provider ServiceProvider) *Server {
	return &Server{
		provider:  provider,
		startTime: time.Now(),
	}
}

// Serve accepts incoming stream connections on listener and dispatches RPC requests.
func (s *Server) Serve(listener net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("server is closed")
	}
	s.listeners = append(s.listeners, listener)
	s.mu.Unlock()

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.RLock()
			closed := s.closed
			s.mu.RUnlock()
			if closed {
				return nil
			}
			return fmt.Errorf("accept error: %w", err)
		}
		go s.handleConnection(conn)
	}
}

// Close shuts down all active server listeners.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var errs []error
	for _, l := range s.listeners {
		if err := l.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		payload, err := ReadFrame(conn)
		if err != nil {
			if err != io.EOF {
				// Connection terminated or closed
			}
			return
		}

		var req RPCRequest
		if err := DecodeJSON(payload, &req); err != nil {
			resp := RPCResponse{
				Success: false,
				Error:   fmt.Sprintf("invalid JSON payload: %v", err),
			}
			respBytes, _ := EncodeJSON(resp)
			_ = WriteFrame(conn, respBytes)
			continue
		}

		resp := s.dispatchCommand(&req)
		respBytes, err := EncodeJSON(resp)
		if err != nil {
			respBytes, _ = EncodeJSON(RPCResponse{
				ID:      req.ID,
				Success: false,
				Error:   fmt.Sprintf("failed encoding response: %v", err),
			})
		}
		if err := WriteFrame(conn, respBytes); err != nil {
			return
		}
	}
}

func (s *Server) dispatchCommand(req *RPCRequest) RPCResponse {
	switch req.Command {
	case "ping":
		return RPCResponse{
			ID:      req.ID,
			Success: true,
			Data:    "PONG",
		}

	case "list_services", "ps":
		if s.provider == nil {
			return RPCResponse{ID: req.ID, Success: false, Error: "no service provider configured"}
		}
		services := s.provider.ListServices()
		return RPCResponse{
			ID:      req.ID,
			Success: true,
			Data:    services,
		}

	case "start_service", "start":
		if s.provider == nil {
			return RPCResponse{ID: req.ID, Success: false, Error: "no service provider configured"}
		}
		if req.Service == "" {
			return RPCResponse{ID: req.ID, Success: false, Error: "missing service name"}
		}
		if err := s.provider.StartService(req.Service); err != nil {
			return RPCResponse{ID: req.ID, Success: false, Error: err.Error()}
		}
		return RPCResponse{ID: req.ID, Success: true, Data: fmt.Sprintf("service %s started successfully", req.Service)}

	case "stop_service", "stop":
		if s.provider == nil {
			return RPCResponse{ID: req.ID, Success: false, Error: "no service provider configured"}
		}
		if req.Service == "" {
			return RPCResponse{ID: req.ID, Success: false, Error: "missing service name"}
		}
		if err := s.provider.StopService(req.Service); err != nil {
			return RPCResponse{ID: req.ID, Success: false, Error: err.Error()}
		}
		return RPCResponse{ID: req.ID, Success: true, Data: fmt.Sprintf("service %s stopped successfully", req.Service)}

	case "get_logs", "logs":
		if s.provider == nil {
			return RPCResponse{ID: req.ID, Success: false, Error: "no service provider configured"}
		}
		if req.Service == "" {
			return RPCResponse{ID: req.ID, Success: false, Error: "missing service name"}
		}
		logs, err := s.provider.GetServiceLogs(req.Service)
		if err != nil {
			return RPCResponse{ID: req.ID, Success: false, Error: err.Error()}
		}
		return RPCResponse{ID: req.ID, Success: true, Data: logs}

	case "get_metrics", "metrics":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		numServices := 0
		if s.provider != nil {
			numServices = len(s.provider.ListServices())
		}

		metrics := SystemMetrics{
			UptimeSeconds: time.Since(s.startTime).Seconds(),
			NumGoroutine:  runtime.NumGoroutine(),
			MemoryAlloc:   m.Alloc,
			MemorySys:     m.Sys,
			NumServices:   numServices,
		}
		return RPCResponse{ID: req.ID, Success: true, Data: metrics}

	default:
		return RPCResponse{
			ID:      req.ID,
			Success: false,
			Error:   fmt.Sprintf("unknown command: %q", req.Command),
		}
	}
}
