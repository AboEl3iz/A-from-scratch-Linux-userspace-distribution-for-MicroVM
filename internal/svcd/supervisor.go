package svcd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ProcessState int

const (
	StateStopped ProcessState = iota
	StateStarting
	StateRunning
	StateStopping
	StateTerminated
)

func (s ProcessState) String() string {
	switch s {
	case StateStopped:
		return "STOPPED"
	case StateStarting:
		return "STARTING"
	case StateRunning:
		return "RUNNING"
	case StateStopping:
		return "STOPPING"
	case StateTerminated:
		return "TERMINATED"
	default:
		return "UNKNOWN"
	}
}

// ManagedService represents a runtime instance of a service.
type ManagedService struct {
	Spec         *ServiceSpec
	State        ProcessState
	Cmd          *exec.Cmd
	CGroupMgr        *CGroupManager
	mu               sync.Mutex
	restartCount     int
	exitCodeRecorded bool
	recordedExitCode int
}

func (ms *ManagedService) RecordExitStatus(exitCode int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.exitCodeRecorded = true
	ms.recordedExitCode = exitCode
}

// NewManagedService creates a new supervisor handle for a service.
func NewManagedService(spec *ServiceSpec, cgm *CGroupManager) *ManagedService {
	return &ManagedService{
		Spec:      spec,
		State:     StateStopped,
		CGroupMgr: cgm,
	}
}

// Start launches the service process, attaches cgroup, and begins monitoring stdout/stderr.
func (ms *ManagedService) Start() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.State == StateRunning {
		return fmt.Errorf("service %s is already running", ms.Spec.Name)
	}

	ms.State = StateStarting

	execPath := ms.Spec.Exec
	execArgs := ms.Spec.Args

	// Pre-flight check: Verify target executable exists before launching secd wrapper or process
	targetCheckPath := execPath
	if ms.Spec.RootDir != "" {
		targetCheckPath = filepath.Join(ms.Spec.RootDir, execPath)
	}
	if _, err := os.Stat(targetCheckPath); err != nil {
		buildFallback := filepath.Join("build", filepath.Base(execPath))
		if _, errBuild := os.Stat(buildFallback); errBuild == nil {
			execPath = buildFallback
			targetCheckPath = buildFallback
		} else if _, errPath := exec.LookPath(execPath); errPath != nil {
			ms.State = StateStopped
			return fmt.Errorf("executable %q not found on rootfs (import container layer via 'karim import')", execPath)
		}
	}

	// Check if karim-secd isolation wrapper binary exists
	secdBin := "/sbin/karim-secd"
	if _, err := os.Stat(secdBin); err != nil {
		if _, err2 := os.Stat("build/karim-secd"); err2 == nil {
			secdBin = "build/karim-secd"
		} else {
			secdBin = ""
		}
	}

	if secdBin != "" && ms.Spec.SeccompProfile != "unrestricted" && ms.Spec.SeccompProfile != "none" && ms.Spec.SeccompProfile != "disabled" {
		secdArgs := []string{
			"-profile", ms.Spec.SeccompProfile,
			"-exec", execPath,
		}
		if len(ms.Spec.CapabilitiesAdd) > 0 {
			secdArgs = append(secdArgs, "-caps-add", strings.Join(ms.Spec.CapabilitiesAdd, ","))
		}
		if len(ms.Spec.CapabilitiesDrop) > 0 {
			secdArgs = append(secdArgs, "-caps-drop", strings.Join(ms.Spec.CapabilitiesDrop, ","))
		}
		if !ms.Spec.NoNewPrivs {
			secdArgs = append(secdArgs, "-no-new-privs=false")
		}
		secdArgs = append(secdArgs, "--")
		secdArgs = append(secdArgs, execArgs...)

		execPath = secdBin
		execArgs = secdArgs
	}

	cmd := exec.Command(execPath, execArgs...)
	if ms.Spec.Directory != "" {
		cmd.Dir = ms.Spec.Directory
	}
	defaultEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"LD_LIBRARY_PATH=/lib:/usr/lib:/usr/local/lib:/lib64:/usr/lib64:/usr/local/lib64",
		"HOME=/root",
		"TMPDIR=/tmp",
	}
	cmd.Env = append(defaultEnv, ms.Spec.Env...)

	// Wire RootDir into SysProcAttr.Chroot so that exec paths like /bin/sh resolve
	// inside the container layer mount rather than the host merged rootfs.
	procAttr := SetupChildProcAttr()
	if ms.Spec.RootDir != "" {
		procAttr.Chroot = ms.Spec.RootDir
		// When chroot is active, the working directory must be relative to the new root.
		if cmd.Dir == "" {
			cmd.Dir = "/"
		}
	}
	cmd.SysProcAttr = procAttr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		ms.State = StateStopped
		return fmt.Errorf("failed stdout pipe for %s: %w", ms.Spec.Name, err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		ms.State = StateStopped
		return fmt.Errorf("failed stderr pipe for %s: %w", ms.Spec.Name, err)
	}

	if err := cmd.Start(); err != nil {
		ms.State = StateStopped
		return fmt.Errorf("failed to start service process %s: %w", ms.Spec.Name, err)
	}

	ms.Cmd = cmd
	ms.State = StateRunning
	fmt.Printf("[karim-svcd] Service %s started (PID %d)\n", ms.Spec.Name, cmd.Process.Pid)

	// Create and attach to service cgroup
	if ms.CGroupMgr != nil {
		if _, err := ms.CGroupMgr.CreateServiceCGroup(ms.Spec); err == nil {
			_ = ms.CGroupMgr.AttachProcess(ms.Spec.Name, cmd.Process.Pid)
		}
	}

	// Stream stdout and stderr logs in background goroutines
	go streamLogs(fmt.Sprintf("[karim-svcd | %s] ", ms.Spec.Name), stdoutPipe)
	go streamLogs(fmt.Sprintf("[karim-svcd | %s | ERR] ", ms.Spec.Name), stderrPipe)

	// Monitor process completion asynchronously
	go ms.monitor()

	return nil
}

// Stop gracefully signals process termination.
func (ms *ManagedService) Stop() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.State != StateRunning || ms.Cmd == nil || ms.Cmd.Process == nil {
		return nil
	}

	ms.State = StateStopping
	fmt.Printf("[karim-svcd] Stopping service %s (PID %d)...\n", ms.Spec.Name, ms.Cmd.Process.Pid)

	// Send SIGTERM
	if err := ms.Cmd.Process.Signal(os.Interrupt); err != nil {
		_ = ms.Cmd.Process.Kill()
	}
	return nil
}

func (ms *ManagedService) monitor() {
	err := ms.Cmd.Wait()

	ms.mu.Lock()
	ms.State = StateTerminated
	exitCode := 0
	if ms.exitCodeRecorded {
		exitCode = ms.recordedExitCode
		if exitCode != 0 {
			fmt.Printf("[karim-svcd] Service %s exited with status %d\n", ms.Spec.Name, exitCode)
		} else {
			fmt.Printf("[karim-svcd] Service %s exited cleanly (code 0)\n", ms.Spec.Name)
		}
	} else if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			fmt.Printf("[karim-svcd] Service %s exited with status %d: %v\n", ms.Spec.Name, exitCode, err)
		} else if strings.Contains(err.Error(), "no child processes") {
			exitCode = 1
			fmt.Printf("[karim-svcd] Service %s exited (status captured by supervisor)\n", ms.Spec.Name)
		} else {
			exitCode = 1
			fmt.Printf("[karim-svcd] Service %s exited: %v\n", ms.Spec.Name, err)
		}
	} else {
		fmt.Printf("[karim-svcd] Service %s exited cleanly (code 0)\n", ms.Spec.Name)
	}
	ms.mu.Unlock()

	// Handle restart policies
	shouldRestart := false
	switch ms.Spec.Restart {
	case "always":
		shouldRestart = true
	case "on-failure":
		if exitCode != 0 {
			shouldRestart = true
		}
	}

	if !shouldRestart {
		return
	}

	// Crashloop protection: enforce MaxRestarts limit.
	// MaxRestarts == 0 means unlimited (backwards-compatible default).
	ms.mu.Lock()
	ms.restartCount++
	count := ms.restartCount
	ms.mu.Unlock()

	if ms.Spec.MaxRestarts > 0 && count > ms.Spec.MaxRestarts {
		fmt.Printf("[karim-svcd] Service %s exceeded max_restarts=%d (attempt %d). Will not restart.\n",
			ms.Spec.Name, ms.Spec.MaxRestarts, count)
		return
	}

	// Exponential backoff: 1s, 2s, 4s, 8s, ... up to 30s cap.
	// This prevents a permanently broken binary (e.g. exec ENOENT) from
	// burning CPU in a tight loop and flooding logs.
	backoff := time.Duration(1<<min(count-1, 5)) * time.Second // 1s to 32s, cap at 30s
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	fmt.Printf("[karim-svcd] Restart policy '%s' active for %s (attempt %d/%s). Restarting in %v...\n",
		ms.Spec.Restart, ms.Spec.Name, count, maxRestartsLabel(ms.Spec.MaxRestarts), backoff)

	time.Sleep(backoff)
	_ = ms.Start()
}

func maxRestartsLabel(max int) string {
	if max == 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", max)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}



func streamLogs(prefix string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Printf("%s%s\n", prefix, scanner.Text())
	}
}
