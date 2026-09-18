package snapshot

import (
	"fmt"
	"time"

	"karim-microvm-os/internal/qmp"
	"karim-microvm-os/internal/vsockd"
)

// SnapshotConfig configures the microVM QMP snapshot orchestrator.
type SnapshotConfig struct {
	QMPSocketPath string // Path to QMP Unix socket (e.g. /tmp/qmp.sock)
	VSockTarget   string // Address of guest vsockd (e.g. vsock://3:1024 or unix:///run/karim/vsock.sock)
	PauseVM       bool   // If true, issue QMP 'stop' before savevm and 'cont' afterwards
}

// SnapshotResult contains execution telemetry for a snapshot save/restore operation.
type SnapshotResult struct {
	Tag       string        `json:"tag"`
	Quiesced  bool          `json:"quiesced"`
	Output    string        `json:"output"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// Orchestrator coordinates hypervisor state saving/restoring with guest VFS quiescing hooks.
type Orchestrator struct {
	Config SnapshotConfig
}

// NewOrchestrator creates a new snapshot orchestrator instance.
func NewOrchestrator(cfg SnapshotConfig) *Orchestrator {
	if cfg.QMPSocketPath == "" {
		cfg.QMPSocketPath = "/tmp/qmp.sock"
	}
	return &Orchestrator{
		Config: cfg,
	}
}

func sendVSockRPC(target, command string) error {
	if target == "" {
		return nil
	}
	conn, err := vsockd.Dial(target)
	if err != nil {
		return fmt.Errorf("failed connecting to vsock target %s: %w", target, err)
	}
	defer conn.Close()

	reqBytes, err := vsockd.EncodeJSON(vsockd.RPCRequest{
		ID:      fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Command: command,
	})
	if err != nil {
		return err
	}

	if err := vsockd.WriteFrame(conn, reqBytes); err != nil {
		return err
	}

	respBytes, err := vsockd.ReadFrame(conn)
	if err != nil {
		return err
	}

	var resp vsockd.RPCResponse
	if err := vsockd.DecodeJSON(respBytes, &resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("guest RPC %s failed: %s", command, resp.Error)
	}
	return nil
}

// SaveSnapshot performs guest VFS quiesce -> hypervisor pause -> QMP savevm -> hypervisor cont -> guest unquiesce.
func (o *Orchestrator) SaveSnapshot(tag string) (*SnapshotResult, error) {
	if tag == "" {
		return nil, fmt.Errorf("snapshot tag cannot be empty")
	}

	start := time.Now()
	quiesced := false

	// 1. Quiesce guest filesystem buffers over VSOCK if target is configured
	if o.Config.VSockTarget != "" {
		if err := sendVSockRPC(o.Config.VSockTarget, "quiesce"); err == nil {
			quiesced = true
		} else {
			// Log notice but continue with snapshot if vsock is un-reachable
			fmt.Printf("[snapshot-orchestrator] Notice: guest VSOCK quiesce un-reachable (%v). Proceeding with QMP savevm.\n", err)
		}
	}

	// 2. Connect to QMP endpoint
	qmpClient, err := qmp.Dial(o.Config.QMPSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to QMP hypervisor socket %s: %w", o.Config.QMPSocketPath, err)
	}
	defer qmpClient.Close()

	// 3. Pause VM if configured
	if o.Config.PauseVM {
		_ = qmpClient.Stop()
	}

	// 4. Issue QMP savevm command
	out, err := qmpClient.SaveVM(tag)

	// 5. Resume VM if paused
	if o.Config.PauseVM {
		_ = qmpClient.Cont()
	}

	// 6. Unquiesce guest if previously quiesced
	if quiesced {
		_ = sendVSockRPC(o.Config.VSockTarget, "unquiesce")
	}

	if err != nil {
		return nil, fmt.Errorf("QMP savevm failed for tag %q: %w", tag, err)
	}

	return &SnapshotResult{
		Tag:       tag,
		Quiesced:  quiesced,
		Output:    out,
		Duration:  time.Since(start),
		Timestamp: start,
	}, nil
}

// RestoreSnapshot restores microVM state from a named QMP snapshot and unquiesces guest execution.
func (o *Orchestrator) RestoreSnapshot(tag string) (*SnapshotResult, error) {
	if tag == "" {
		return nil, fmt.Errorf("snapshot tag cannot be empty")
	}

	start := time.Now()

	// 1. Connect to QMP endpoint
	qmpClient, err := qmp.Dial(o.Config.QMPSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to QMP hypervisor socket %s: %w", o.Config.QMPSocketPath, err)
	}
	defer qmpClient.Close()

	// 2. Execute QMP loadvm
	out, err := qmpClient.LoadVM(tag)
	if err != nil {
		return nil, fmt.Errorf("QMP loadvm failed for tag %q: %w", tag, err)
	}

	// 3. Resume CPU execution
	_ = qmpClient.Cont()

	// 4. Notify guest of restore completion if VSOCK is connected
	if o.Config.VSockTarget != "" {
		_ = sendVSockRPC(o.Config.VSockTarget, "unquiesce")
	}

	return &SnapshotResult{
		Tag:       tag,
		Quiesced:  false,
		Output:    out,
		Duration:  time.Since(start),
		Timestamp: start,
	}, nil
}

// ListSnapshots queries active snapshots recorded in QEMU block devices.
func (o *Orchestrator) ListSnapshots() ([]qmp.SnapshotInfo, error) {
	qmpClient, err := qmp.Dial(o.Config.QMPSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to QMP socket %s: %w", o.Config.QMPSocketPath, err)
	}
	defer qmpClient.Close()

	return qmpClient.ListSnapshots()
}

// DeleteSnapshot deletes a named state snapshot from QEMU block storage.
func (o *Orchestrator) DeleteSnapshot(tag string) error {
	if tag == "" {
		return fmt.Errorf("snapshot tag cannot be empty")
	}
	qmpClient, err := qmp.Dial(o.Config.QMPSocketPath)
	if err != nil {
		return fmt.Errorf("failed connecting to QMP socket %s: %w", o.Config.QMPSocketPath, err)
	}
	defer qmpClient.Close()

	_, err = qmpClient.DelVM(tag)
	return err
}

// QueryStatus queries microVM CPU execution status via QMP.
func (o *Orchestrator) QueryStatus() (*qmp.VMStatus, error) {
	qmpClient, err := qmp.Dial(o.Config.QMPSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to QMP socket %s: %w", o.Config.QMPSocketPath, err)
	}
	defer qmpClient.Close()

	return qmpClient.QueryStatus()
}
