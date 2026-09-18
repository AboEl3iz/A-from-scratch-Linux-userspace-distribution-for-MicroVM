package qmp

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupMockQMPServer(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "qmp_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}

	sockPath := filepath.Join(tmpDir, "qmp.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed listening on unix socket: %v", err)
	}

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockQMPConnection(t, conn)
		}
	}()

	cleanup := func() {
		_ = listener.Close()
		_ = os.RemoveAll(tmpDir)
		<-doneChan
	}

	return sockPath, cleanup
}

func handleMockQMPConnection(t *testing.T, conn net.Conn) {
	defer conn.Close()

	// 1. Send QMP Greeting
	greeting := `{"QMP": {"version": {"qemu": {"major": 8, "minor": 2, "micro": 0}}, "capabilities": []}}` + "\n"
	_, _ = conn.Write([]byte(greeting))

	reader := bufio.NewReader(conn)
	isCapabilitiesOk := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req qmpCommand
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := `{"error": {"class": "GenericError", "desc": "invalid json"}}` + "\n"
			_, _ = conn.Write([]byte(resp))
			continue
		}

		if req.Execute == "qmp_capabilities" {
			isCapabilitiesOk = true
			_, _ = conn.Write([]byte(`{"return": {}}` + "\n"))
			continue
		}

		if !isCapabilitiesOk {
			_, _ = conn.Write([]byte(`{"error": {"class": "CommandNotFound", "desc": "capabilities not negotiated"}}` + "\n"))
			continue
		}

		switch req.Execute {
		case "stop":
			_, _ = conn.Write([]byte(`{"return": {}}` + "\n"))

		case "cont":
			_, _ = conn.Write([]byte(`{"return": {}}` + "\n"))

		case "query-status":
			statusJSON := `{"return": {"running": true, "singlestep": false, "status": "running"}}` + "\n"
			_, _ = conn.Write([]byte(statusJSON))

		case "human-monitor-command":
			cmdLine, _ := req.Arguments["command-line"].(string)
			if strings.HasPrefix(cmdLine, "savevm ") {
				tag := strings.TrimPrefix(cmdLine, "savevm ")
				_, _ = conn.Write([]byte(`{"return": "Snapshot ` + tag + ` created successfully\n"}` + "\n"))
			} else if strings.HasPrefix(cmdLine, "loadvm ") {
				tag := strings.TrimPrefix(cmdLine, "loadvm ")
				_, _ = conn.Write([]byte(`{"return": "Snapshot ` + tag + ` restored successfully\n"}` + "\n"))
			} else if strings.HasPrefix(cmdLine, "delvm ") {
				tag := strings.TrimPrefix(cmdLine, "delvm ")
				_, _ = conn.Write([]byte(`{"return": "Snapshot ` + tag + ` deleted successfully\n"}` + "\n"))
			} else if cmdLine == "info snapshots" {
				infoStr := "Snapshot devices: drive0\n" +
					"Snapshot list (drives):\n" +
					"ID        TAG                 VM SIZE       DATE       VM CLOCK\n" +
					"1         test-snap-1            42M 2026-09-17 12:00:00   00:00:15.123\n" +
					"2         test-snap-2            48M 2026-09-17 12:05:00   00:01:30.456\n"
				respBytes, _ := json.Marshal(qmpResponse{Return: infoStr})
				respBytes = append(respBytes, '\n')
				_, _ = conn.Write(respBytes)
			} else if cmdLine == "invalid" {
				_, _ = conn.Write([]byte(`{"error": {"class": "GenericError", "desc": "Unknown command invalid"}}` + "\n"))
			} else {
				_, _ = conn.Write([]byte(`{"return": "OK"}` + "\n"))
			}

		default:
			_, _ = conn.Write([]byte(`{"error": {"class": "CommandNotFound", "desc": "unknown command"}}` + "\n"))
		}
	}
}

func TestQMPClientHandshakeAndStatus(t *testing.T) {
	sockPath, cleanup := setupMockQMPServer(t)
	defer cleanup()

	client, err := DialTimeout(sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("DialTimeout failed: %v", err)
	}
	defer client.Close()

	// Test QueryStatus
	status, err := client.QueryStatus()
	if err != nil {
		t.Fatalf("QueryStatus failed: %v", err)
	}
	if !status.Running {
		t.Errorf("expected running=true, got %v", status.Running)
	}
	if status.Status != "running" {
		t.Errorf("expected status=running, got %s", status.Status)
	}

	// Test Stop and Cont
	if err := client.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	if err := client.Cont(); err != nil {
		t.Errorf("Cont failed: %v", err)
	}
}

func TestQMPSnapshotOperations(t *testing.T) {
	sockPath, cleanup := setupMockQMPServer(t)
	defer cleanup()

	client, err := Dial(sockPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	// Test SaveVM
	out, err := client.SaveVM("snap-alpha")
	if err != nil {
		t.Fatalf("SaveVM failed: %v", err)
	}
	if !strings.Contains(out, "snap-alpha") {
		t.Errorf("expected output to mention snap-alpha, got: %s", out)
	}

	// Test LoadVM
	out, err = client.LoadVM("snap-alpha")
	if err != nil {
		t.Fatalf("LoadVM failed: %v", err)
	}
	if !strings.Contains(out, "snap-alpha") {
		t.Errorf("expected output to mention snap-alpha, got: %s", out)
	}

	// Test DelVM
	out, err = client.DelVM("snap-alpha")
	if err != nil {
		t.Fatalf("DelVM failed: %v", err)
	}
	if !strings.Contains(out, "snap-alpha") {
		t.Errorf("expected output to mention snap-alpha, got: %s", out)
	}

	// Test ListSnapshots
	snapshots, err := client.ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
	if snapshots[0].Tag != "test-snap-1" || snapshots[0].VMSize != "42M" {
		t.Errorf("unexpected snapshot 0: %+v", snapshots[0])
	}
	if snapshots[1].Tag != "test-snap-2" || snapshots[1].VMSize != "48M" {
		t.Errorf("unexpected snapshot 1: %+v", snapshots[1])
	}
}

func TestQMPErrorHandling(t *testing.T) {
	sockPath, cleanup := setupMockQMPServer(t)
	defer cleanup()

	client, err := Dial(sockPath)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	_, err = client.HumanMonitorCommand("invalid")
	if err == nil {
		t.Fatal("expected error for invalid HMP command, got nil")
	}

	qerr, ok := err.(*QMPError)
	if !ok {
		t.Fatalf("expected *QMPError type, got %T: %v", err, err)
	}
	if qerr.Class != "GenericError" {
		t.Errorf("expected GenericError class, got %s", qerr.Class)
	}
}

func TestParseSnapshotList(t *testing.T) {
	rawOutput := `
Snapshot devices: drive0
Snapshot list (drives):
ID        TAG                 VM SIZE       DATE       VM CLOCK
1         v1.0                   15M 2026-09-17 10:00:00   00:00:05.123
2         v1.1                   22M 2026-09-17 11:30:00   00:00:45.678
`

	list := ParseSnapshotList(rawOutput)
	if len(list) != 2 {
		t.Fatalf("expected 2 parsed items, got %d", len(list))
	}
	if list[0].Tag != "v1.0" || list[0].ID != "1" || list[0].VMSize != "15M" {
		t.Errorf("parsed snapshot 0 mismatch: %+v", list[0])
	}
	if list[1].Tag != "v1.1" || list[1].ID != "2" || list[1].VMSize != "22M" {
		t.Errorf("parsed snapshot 1 mismatch: %+v", list[1])
	}
}
