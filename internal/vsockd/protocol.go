package vsockd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const MaxFrameSize = 10 * 1024 * 1024 // 10MB max RPC payload size

// RPCRequest represents an incoming command frame sent to karim-vsockd.
type RPCRequest struct {
	ID      string   `json:"id"`
	Command string   `json:"command"` // "ping", "list_services", "start_service", "stop_service", "get_logs", "get_metrics", "load_service", "run_service"
	Service string   `json:"service,omitempty"`
	Args    []string `json:"args,omitempty"`
	// Payload carries inline TOML config or JSON spec for load_service / run_service commands.
	Payload string `json:"payload,omitempty"`
}

// RPCResponse represents an outgoing result frame returned by karim-vsockd.
type RPCResponse struct {
	ID      string      `json:"id"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ServiceInfo represents telemetry and configuration status for a managed service.
type ServiceInfo struct {
	Name           string   `json:"name"`
	State          string   `json:"state"`
	PID            int      `json:"pid"`
	Exec           string   `json:"exec"`
	MemoryLimit    int64    `json:"memory_limit"`
	CPUQuota       float64  `json:"cpu_quota"`
	Restart        string   `json:"restart"`
	SeccompProfile string   `json:"seccomp_profile"`
	After          []string `json:"after"`
}

// SystemMetrics represents overall guest VM performance metrics.
type SystemMetrics struct {
	UptimeSeconds float64 `json:"uptime_seconds"`
	NumGoroutine  int     `json:"num_goroutine"`
	MemoryAlloc   uint64  `json:"memory_alloc_bytes"`
	MemorySys     uint64  `json:"memory_sys_bytes"`
	NumServices   int     `json:"num_services"`
}

// WriteFrame sends a 4-byte big-endian length-prefixed payload stream to writer.
func WriteFrame(writer io.Writer, payload []byte) error {
	length := uint32(len(payload))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)

	if _, err := writer.Write(header); err != nil {
		return fmt.Errorf("failed writing frame length header: %w", err)
	}
	if length > 0 {
		if _, err := writer.Write(payload); err != nil {
			return fmt.Errorf("failed writing frame payload: %w", err)
		}
	}
	return nil
}

// ReadFrame reads a 4-byte big-endian length-prefixed payload stream from reader.
func ReadFrame(reader io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)
	if length > MaxFrameSize {
		return nil, fmt.Errorf("frame size %d exceeds maximum allowed limit (%d bytes)", length, MaxFrameSize)
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, fmt.Errorf("failed reading frame payload: %w", err)
		}
	}
	return payload, nil
}

// EncodeJSON encodes v into JSON bytes.
func EncodeJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// DecodeJSON decodes JSON data bytes into v.
func DecodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
