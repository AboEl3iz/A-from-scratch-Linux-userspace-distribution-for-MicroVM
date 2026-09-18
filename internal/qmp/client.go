package qmp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// QMPError represents an error returned by QEMU Monitor Protocol.
type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

func (e *QMPError) Error() string {
	return fmt.Sprintf("QMP Error [%s]: %s", e.Class, e.Desc)
}

type qmpGreeting struct {
	QMP struct {
		Version      interface{} `json:"version"`
		Capabilities []string    `json:"capabilities"`
	} `json:"QMP"`
}

type qmpCommand struct {
	Execute   string                 `json:"execute"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type qmpResponse struct {
	Return interface{} `json:"return,omitempty"`
	Error  *QMPError   `json:"error,omitempty"`
	Event  string      `json:"event,omitempty"`
}

// VMStatus captures QEMU execution status from query-status command.
type VMStatus struct {
	Running    bool   `json:"running"`
	Singlestep bool   `json:"singlestep"`
	Status     string `json:"status"`
}

// SnapshotInfo describes a microVM state snapshot recorded in QEMU storage.
type SnapshotInfo struct {
	ID      string `json:"id"`
	Tag     string `json:"tag"`
	VMSize  string `json:"vm_size"`
	Date    string `json:"date"`
	VMClock string `json:"vm_clock"`
}

// Client manages a JSON RPC connection to the QEMU Monitor Protocol socket.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	closed bool
}

// Dial connects to a QMP endpoint (Unix socket path, unix://, or tcp://) and performs capability handshake.
func Dial(target string) (*Client, error) {
	return DialTimeout(target, 5*time.Second)
}

// DialTimeout connects to a QMP endpoint with a specified timeout.
func DialTimeout(target string, timeout time.Duration) (*Client, error) {
	network, addr := parseTarget(target)

	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to QMP socket %s (%s): %w", target, network, err)
	}

	c := &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}

	if err := c.handshake(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("QMP capability handshake failed: %w", err)
	}

	return c, nil
}

func parseTarget(target string) (string, string) {
	if strings.HasPrefix(target, "unix://") {
		return "unix", strings.TrimPrefix(target, "unix://")
	}
	if strings.HasPrefix(target, "tcp://") {
		return "tcp", strings.TrimPrefix(target, "tcp://")
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "./") {
		return "unix", target
	}
	return "unix", target
}

// DialConn initializes a QMP client from an existing net.Conn (useful for testing or custom transports).
func DialConn(conn net.Conn) (*Client, error) {
	c := &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}
	if err := c.handshake(); err != nil {
		return nil, fmt.Errorf("QMP capability handshake failed: %w", err)
	}
	return c, nil
}

func (c *Client) handshake() error {
	// Read QMP greeting banner
	line, err := c.readLine()
	if err != nil {
		return fmt.Errorf("failed reading QMP greeting: %w", err)
	}

	var greeting qmpGreeting
	if err := json.Unmarshal([]byte(line), &greeting); err != nil {
		return fmt.Errorf("invalid QMP greeting banner payload: %w", err)
	}

	// Negotiate QMP capabilities
	resp, err := c.Execute("qmp_capabilities", nil)
	if err != nil {
		return fmt.Errorf("qmp_capabilities failed: %w", err)
	}
	_ = resp
	return nil
}

func (c *Client) readLine() (string, error) {
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Ignore asynchronous event notifications (e.g., {"event": "STOP", ...}) during command response reading
		if strings.Contains(line, `"event":`) && !strings.Contains(line, `"return":`) && !strings.Contains(line, `"error":`) {
			continue
		}
		return line, nil
	}
}

// Execute dispatches a JSON QMP command and waits for response return or error.
func (c *Client) Execute(cmd string, args map[string]interface{}) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("qmp client is closed")
	}

	req := qmpCommand{
		Execute:   cmd,
		Arguments: args,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling QMP command %s: %w", cmd, err)
	}
	payload = append(payload, '\n')

	if _, err := c.conn.Write(payload); err != nil {
		return nil, fmt.Errorf("failed writing to QMP socket: %w", err)
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return nil, fmt.Errorf("failed reading QMP command response: %w", err)
		}

		var resp qmpResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return nil, fmt.Errorf("failed unmarshaling QMP response line %q: %w", line, err)
		}

		if resp.Event != "" && resp.Return == nil && resp.Error == nil {
			// Skip asynchronous QMP event
			continue
		}

		if resp.Error != nil {
			return nil, resp.Error
		}

		return resp.Return, nil
	}
}

// Stop pauses VM CPU execution.
func (c *Client) Stop() error {
	_, err := c.Execute("stop", nil)
	return err
}

// Cont resumes VM CPU execution.
func (c *Client) Cont() error {
	_, err := c.Execute("cont", nil)
	return err
}

// QueryStatus returns the current CPU execution state of the QEMU microVM.
func (c *Client) QueryStatus() (*VMStatus, error) {
	res, err := c.Execute("query-status", nil)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	var status VMStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("failed parsing query-status response: %w", err)
	}

	return &status, nil
}

// HumanMonitorCommand executes a legacy QEMU HMP command via QMP wrapper and returns raw console output.
func (c *Client) HumanMonitorCommand(commandLine string) (string, error) {
	res, err := c.Execute("human-monitor-command", map[string]interface{}{
		"command-line": commandLine,
	})
	if err != nil {
		return "", err
	}

	if outputStr, ok := res.(string); ok {
		return outputStr, nil
	}
	return fmt.Sprintf("%v", res), nil
}

// SaveVM creates a named state snapshot in QEMU storage.
func (c *Client) SaveVM(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("snapshot tag name cannot be empty")
	}
	return c.HumanMonitorCommand(fmt.Sprintf("savevm %s", name))
}

// LoadVM restores a microVM to a named state snapshot.
func (c *Client) LoadVM(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("snapshot tag name cannot be empty")
	}
	return c.HumanMonitorCommand(fmt.Sprintf("loadvm %s", name))
}

// DelVM deletes a named state snapshot from QEMU storage.
func (c *Client) DelVM(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("snapshot tag name cannot be empty")
	}
	return c.HumanMonitorCommand(fmt.Sprintf("delvm %s", name))
}

// ListSnapshots returns all active state snapshots saved in QEMU block devices.
func (c *Client) ListSnapshots() ([]SnapshotInfo, error) {
	output, err := c.HumanMonitorCommand("info snapshots")
	if err != nil {
		return nil, err
	}
	return ParseSnapshotList(output), nil
}

// ParseSnapshotList parses text output from HMP 'info snapshots' command.
func ParseSnapshotList(output string) []SnapshotInfo {
	var snapshots []SnapshotInfo
	lines := strings.Split(output, "\n")

	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "ID") && strings.Contains(trimmed, "TAG") && strings.Contains(trimmed, "VM SIZE") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 5 {
			sn := SnapshotInfo{
				ID:      fields[0],
				Tag:     fields[1],
				VMSize:  fields[2],
				Date:    fields[3],
				VMClock: fields[4],
			}
			if len(fields) >= 6 {
				sn.Date = fields[3] + " " + fields[4]
				sn.VMClock = fields[5]
			}
			snapshots = append(snapshots, sn)
		}
	}
	return snapshots
}

// Close closes the underlying QMP network connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// IsQMPSocketAvailable checks if a QMP socket exists and is connectable.
func IsQMPSocketAvailable(socketPath string) bool {
	network, addr := parseTarget(socketPath)
	if network == "unix" {
		if _, err := os.Stat(addr); err != nil {
			return false
		}
	}
	conn, err := net.DialTimeout(network, addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
