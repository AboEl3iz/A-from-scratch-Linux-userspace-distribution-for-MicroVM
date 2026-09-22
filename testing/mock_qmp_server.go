package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"karim-microvm-os/internal/vsockd"
)

type mockVSock struct {
	quiesced bool
}

func (m *mockVSock) ListServices() []*vsockd.ServiceInfo        { return nil }
func (m *mockVSock) StartService(name string) error             { return nil }
func (m *mockVSock) StopService(name string) error              { return nil }
func (m *mockVSock) GetServiceLogs(name string) (string, error) { return "", nil }
func (m *mockVSock) Quiesce() error                             { m.quiesced = true; return nil }
func (m *mockVSock) Unquiesce() error                           { m.quiesced = false; return nil }
func (m *mockVSock) LoadService(configTOML string) error        { return nil }


type mockQMPServer struct {
	mu        sync.Mutex
	snapshots map[string]string
}

func newMockQMPServer() *mockQMPServer {
	return &mockQMPServer{
		snapshots: make(map[string]string),
	}
}

func (m *mockQMPServer) serve(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go m.handleConn(conn)
	}
}

func (m *mockQMPServer) handleConn(conn net.Conn) {
	defer conn.Close()

	greeting := `{"QMP": {"version": {"qemu": {"major": 8, "minor": 2, "micro": 0}}, "capabilities": []}}` + "\n"
	_, _ = conn.Write([]byte(greeting))

	reader := bufio.NewReader(conn)
	isCaps := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req map[string]interface{}
		_ = json.Unmarshal([]byte(line), &req)
		cmd, _ := req["execute"].(string)

		if cmd == "qmp_capabilities" {
			isCaps = true
			_, _ = conn.Write([]byte(`{"return": {}}` + "\n"))
			continue
		}

		if !isCaps {
			_, _ = conn.Write([]byte(`{"error": {"class": "CommandNotFound", "desc": "capabilities not negotiated"}}` + "\n"))
			continue
		}

		switch cmd {
		case "stop", "cont":
			_, _ = conn.Write([]byte(`{"return": {}}` + "\n"))

		case "query-status":
			_, _ = conn.Write([]byte(`{"return": {"running": true, "singlestep": false, "status": "running"}}` + "\n"))

		case "human-monitor-command":
			args, _ := req["arguments"].(map[string]interface{})
			cmdLine, _ := args["command-line"].(string)

			m.mu.Lock()
			if strings.HasPrefix(cmdLine, "savevm ") {
				tag := strings.TrimPrefix(cmdLine, "savevm ")
				m.snapshots[tag] = "32M 2026-09-17 12:00:00"
				m.mu.Unlock()
				_, _ = conn.Write([]byte(`{"return": "Snapshot ` + tag + ` saved\n"}` + "\n"))

			} else if strings.HasPrefix(cmdLine, "loadvm ") {
				tag := strings.TrimPrefix(cmdLine, "loadvm ")
				m.mu.Unlock()
				_, _ = conn.Write([]byte(`{"return": "Snapshot ` + tag + ` restored\n"}` + "\n"))

			} else if strings.HasPrefix(cmdLine, "delvm ") {
				tag := strings.TrimPrefix(cmdLine, "delvm ")
				delete(m.snapshots, tag)
				m.mu.Unlock()
				_, _ = conn.Write([]byte(`{"return": "Snapshot ` + tag + ` deleted\n"}` + "\n"))

			} else if cmdLine == "info snapshots" {
				infoStr := "Snapshot devices: drive0\nSnapshot list (drives):\nID        TAG                 VM SIZE       DATE       VM CLOCK\n"
				id := 1
				for tag, date := range m.snapshots {
					infoStr += fmt.Sprintf("%d         %-18s %s   00:00:10.000\n", id, tag, date)
					id++
				}
				m.mu.Unlock()
				respBytes, _ := json.Marshal(map[string]interface{}{"return": infoStr})
				respBytes = append(respBytes, '\n')
				_, _ = conn.Write(respBytes)

			} else {
				m.mu.Unlock()
				_, _ = conn.Write([]byte(`{"return": "OK"}` + "\n"))
			}

		default:
			_, _ = conn.Write([]byte(`{"error": {"class": "CommandNotFound", "desc": "unknown"}}` + "\n"))
		}
	}
}

func main() {
	qmpPath := flag.String("qmp", "/tmp/qmp.sock", "QMP Unix socket path")
	vsockPath := flag.String("vsock", "/tmp/vsock.sock", "VSOCK Unix socket path")
	flag.Parse()

	_ = os.MkdirAll(filepath.Dir(*qmpPath), 0755)
	_ = os.MkdirAll(filepath.Dir(*vsockPath), 0755)

	qmpListener, err := net.Listen("unix", *qmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating QMP listener on %s: %v\n", *qmpPath, err)
		os.Exit(1)
	}
	defer qmpListener.Close()

	vsockListener, err := vsockd.ListenUnix(*vsockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating VSOCK listener on %s: %v\n", *vsockPath, err)
		os.Exit(1)
	}
	defer vsockListener.Close()

	qmpServer := newMockQMPServer()
	go qmpServer.serve(qmpListener)

	vsockServer := vsockd.NewServer(&mockVSock{})
	go func() {
		_ = vsockServer.Serve(vsockListener)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	_ = vsockServer.Close()
}
