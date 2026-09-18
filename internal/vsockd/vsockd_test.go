package vsockd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mockServiceProvider struct {
	services map[string]*ServiceInfo
	logs     map[string]string
}

func newMockServiceProvider() *mockServiceProvider {
	return &mockServiceProvider{
		services: map[string]*ServiceInfo{
			"sample_app": {
				Name:           "sample_app",
				State:          "RUNNING",
				PID:            101,
				Exec:           "/bin/sample_app",
				MemoryLimit:    33554432,
				CPUQuota:       50.0,
				Restart:        "on-failure",
				SeccompProfile: "app-default",
			},
		},
		logs: map[string]string{
			"sample_app": "[karim-svcd | sample_app] Hello from Karim MicroVM OS Service (sample_app)!\n",
		},
	}
}

func (m *mockServiceProvider) ListServices() []*ServiceInfo {
	var list []*ServiceInfo
	for _, s := range m.services {
		list = append(list, s)
	}
	return list
}

func (m *mockServiceProvider) StartService(name string) error {
	s, ok := m.services[name]
	if !ok {
		return fmt.Errorf("service %s not found", name)
	}
	s.State = "RUNNING"
	return nil
}

func (m *mockServiceProvider) StopService(name string) error {
	s, ok := m.services[name]
	if !ok {
		return fmt.Errorf("service %s not found", name)
	}
	s.State = "STOPPED"
	return nil
}

func (m *mockServiceProvider) GetServiceLogs(name string) (string, error) {
	logs, ok := m.logs[name]
	if !ok {
		return "", fmt.Errorf("service %s not found", name)
	}
	return logs, nil
}

func (m *mockServiceProvider) Quiesce() error {
	return nil
}

func (m *mockServiceProvider) Unquiesce() error {
	return nil
}

func TestVSockRPC_ServerClient(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "vsock_test.sock")

	listener, err := ListenUnix(socketPath)
	if err != nil {
		t.Fatalf("failed to create Unix listener: %v", err)
	}

	provider := newMockServiceProvider()
	server := NewServer(provider)

	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	// Wait for server to start listening
	time.Sleep(50 * time.Millisecond)

	// 1. Test Ping RPC
	conn, err := DialUnix(socketPath)
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}

	reqPing, _ := EncodeJSON(RPCRequest{ID: "1", Command: "ping"})
	if err := WriteFrame(conn, reqPing); err != nil {
		t.Fatalf("failed writing ping frame: %v", err)
	}

	respPayload, err := ReadFrame(conn)
	if err != nil {
		t.Fatalf("failed reading ping response: %v", err)
	}

	var respPing RPCResponse
	if err := DecodeJSON(respPayload, &respPing); err != nil {
		t.Fatalf("failed decoding ping response: %v", err)
	}

	if !respPing.Success || respPing.Data != "PONG" {
		t.Fatalf("expected PONG response, got %+v", respPing)
	}

	// 2. Test List Services RPC
	reqList, _ := EncodeJSON(RPCRequest{ID: "2", Command: "list_services"})
	if err := WriteFrame(conn, reqList); err != nil {
		t.Fatalf("failed writing list_services frame: %v", err)
	}

	respListPayload, err := ReadFrame(conn)
	if err != nil {
		t.Fatalf("failed reading list_services response: %v", err)
	}

	var respList RPCResponse
	if err := DecodeJSON(respListPayload, &respList); err != nil {
		t.Fatalf("failed decoding list_services response: %v", err)
	}

	if !respList.Success {
		t.Fatalf("list_services failed: %s", respList.Error)
	}

	rawList, _ := json.Marshal(respList.Data)
	var services []ServiceInfo
	_ = json.Unmarshal(rawList, &services)

	if len(services) != 1 || services[0].Name != "sample_app" {
		t.Fatalf("expected 1 service sample_app, got %+v", services)
	}

	// 3. Test Get Logs RPC
	reqLogs, _ := EncodeJSON(RPCRequest{ID: "3", Command: "logs", Service: "sample_app"})
	if err := WriteFrame(conn, reqLogs); err != nil {
		t.Fatalf("failed writing logs frame: %v", err)
	}

	respLogsPayload, err := ReadFrame(conn)
	if err != nil {
		t.Fatalf("failed reading logs response: %v", err)
	}

	var respLogs RPCResponse
	_ = DecodeJSON(respLogsPayload, &respLogs)
	if !respLogs.Success {
		t.Fatalf("logs request failed: %s", respLogs.Error)
	}

	// 4. Test Metrics RPC
	reqMetrics, _ := EncodeJSON(RPCRequest{ID: "4", Command: "metrics"})
	if err := WriteFrame(conn, reqMetrics); err != nil {
		t.Fatalf("failed writing metrics frame: %v", err)
	}

	respMetricsPayload, err := ReadFrame(conn)
	if err != nil {
		t.Fatalf("failed reading metrics response: %v", err)
	}

	var respMetrics RPCResponse
	_ = DecodeJSON(respMetricsPayload, &respMetrics)
	if !respMetrics.Success {
		t.Fatalf("metrics request failed: %s", respMetrics.Error)
	}

	conn.Close()
	_ = os.Remove(socketPath)
}
