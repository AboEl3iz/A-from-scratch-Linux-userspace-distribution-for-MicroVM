package snapshot

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karim-microvm-os/internal/vsockd"
)

type mockVSockProvider struct {
	quiesced bool
}

func (m *mockVSockProvider) ListServices() []*vsockd.ServiceInfo { return nil }
func (m *mockVSockProvider) StartService(name string) error      { return nil }
func (m *mockVSockProvider) StopService(name string) error       { return nil }
func (m *mockVSockProvider) GetServiceLogs(name string) (string, error) {
	return "", nil
}
func (m *mockVSockProvider) Quiesce() error {
	m.quiesced = true
	return nil
}
func (m *mockVSockProvider) Unquiesce() error {
	m.quiesced = false
	return nil
}
func (m *mockVSockProvider) LoadService(configTOML string) error {
	return nil
}

func setupMockServers(t *testing.T) (string, string, *mockVSockProvider, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	qmpSock := filepath.Join(tmpDir, "qmp.sock")
	qmpListener, err := net.Listen("unix", qmpSock)
	if err != nil {
		t.Fatalf("failed creating mock qmp listener: %v", err)
	}

	go func() {
		for {
			conn, err := qmpListener.Accept()
			if err != nil {
				return
			}
			go handleMockQMP(conn)
		}
	}()

	vsockPath := filepath.Join(tmpDir, "vsock.sock")
	vsockListener, err := vsockd.ListenUnix(vsockPath)
	if err != nil {
		t.Fatalf("failed creating mock vsock listener: %v", err)
	}

	provider := &mockVSockProvider{}
	server := vsockd.NewServer(provider)
	go func() {
		_ = server.Serve(vsockListener)
	}()

	cleanup := func() {
		_ = qmpListener.Close()
		_ = server.Close()
	}

	return qmpSock, "unix://" + vsockPath, provider, cleanup
}

func handleMockQMP(conn net.Conn) {
	defer conn.Close()

	greeting := `{"QMP": {"version": {"qemu": {"major": 8, "minor": 2, "micro": 0}}, "capabilities": []}}` + "\n"
	_, _ = conn.Write([]byte(greeting))

	reader := bufio.NewReader(conn)
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

		switch cmd {
		case "qmp_capabilities", "stop", "cont":
			_, _ = conn.Write([]byte(`{"return": {}}` + "\n"))
		case "query-status":
			_, _ = conn.Write([]byte(`{"return": {"running": true, "singlestep": false, "status": "running"}}` + "\n"))
		case "human-monitor-command":
			args, _ := req["arguments"].(map[string]interface{})
			cmdLine, _ := args["command-line"].(string)
			if strings.HasPrefix(cmdLine, "savevm ") {
				tag := strings.TrimPrefix(cmdLine, "savevm ")
				_, _ = conn.Write([]byte(`{"return": "Saved VM snapshot ` + tag + `"}` + "\n"))
			} else if strings.HasPrefix(cmdLine, "loadvm ") {
				tag := strings.TrimPrefix(cmdLine, "loadvm ")
				_, _ = conn.Write([]byte(`{"return": "Loaded VM snapshot ` + tag + `"}` + "\n"))
			} else if strings.HasPrefix(cmdLine, "delvm ") {
				tag := strings.TrimPrefix(cmdLine, "delvm ")
				_, _ = conn.Write([]byte(`{"return": "Deleted VM snapshot ` + tag + `"}` + "\n"))
			} else if cmdLine == "info snapshots" {
				infoStr := "Snapshot devices: drive0\n" +
					"Snapshot list (drives):\n" +
					"ID        TAG                 VM SIZE       DATE       VM CLOCK\n" +
					"1         test-v1                32M 2026-09-17 12:00:00   00:00:10.000\n"
				respBytes, _ := json.Marshal(map[string]interface{}{"return": infoStr})
				respBytes = append(respBytes, '\n')
				_, _ = conn.Write(respBytes)
			} else {
				_, _ = conn.Write([]byte(`{"return": "OK"}` + "\n"))
			}
		default:
			_, _ = conn.Write([]byte(`{"error": {"class": "GenericError", "desc": "unknown"}}` + "\n"))
		}
	}
}

func TestSnapshotOrchestrationWorkflow(t *testing.T) {
	qmpSock, vsockTarget, _, cleanup := setupMockServers(t)
	defer cleanup()

	// Wait briefly for servers to listen
	time.Sleep(50 * time.Millisecond)

	orch := NewOrchestrator(SnapshotConfig{
		QMPSocketPath: qmpSock,
		VSockTarget:   vsockTarget,
		PauseVM:       true,
	})

	// 1. Test SaveSnapshot
	res, err := orch.SaveSnapshot("snap-v1")
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}
	if res.Tag != "snap-v1" {
		t.Errorf("expected tag snap-v1, got %s", res.Tag)
	}
	if !res.Quiesced {
		t.Errorf("expected quiesced=true, got %v", res.Quiesced)
	}

	// 2. Test ListSnapshots
	snaps, err := orch.ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(snaps) != 1 || snaps[0].Tag != "test-v1" {
		t.Fatalf("expected snapshot test-v1, got %+v", snaps)
	}

	// 3. Test RestoreSnapshot
	restRes, err := orch.RestoreSnapshot("snap-v1")
	if err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	if restRes.Tag != "snap-v1" {
		t.Errorf("expected tag snap-v1 on restore, got %s", restRes.Tag)
	}

	// 4. Test QueryStatus
	status, err := orch.QueryStatus()
	if err != nil {
		t.Fatalf("QueryStatus failed: %v", err)
	}
	if !status.Running {
		t.Errorf("expected running=true, got %v", status.Running)
	}

	// 5. Test DeleteSnapshot
	if err := orch.DeleteSnapshot("snap-v1"); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
}
