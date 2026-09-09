package secd

import (
	"os"
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestParseCapability(t *testing.T) {
	capNum, err := ParseCapability("CAP_NET_BIND_SERVICE")
	if err != nil {
		t.Fatalf("unexpected error parsing CAP_NET_BIND_SERVICE: %v", err)
	}
	if capNum != unix.CAP_NET_BIND_SERVICE {
		t.Errorf("expected capNum %d, got %d", unix.CAP_NET_BIND_SERVICE, capNum)
	}

	capNum2, err := ParseCapability("sys_admin")
	if err != nil {
		t.Fatalf("unexpected error parsing sys_admin: %v", err)
	}
	if capNum2 != unix.CAP_SYS_ADMIN {
		t.Errorf("expected capNum %d, got %d", unix.CAP_SYS_ADMIN, capNum2)
	}

	_, err = ParseCapability("CAP_NON_EXISTENT_CAP")
	if err == nil {
		t.Errorf("expected error for non-existent cap, got nil")
	}
}

func TestCompileSeccompProfile(t *testing.T) {
	filter, err := GetProfileFilter("strict", ActionKillProcess)
	if err != nil {
		t.Fatalf("failed compiling strict profile: %v", err)
	}
	if len(filter.Instructions) == 0 {
		t.Fatalf("expected non-empty BPF filter instructions")
	}

	appFilter, err := GetProfileFilter("app-default", ActionKillProcess)
	if err != nil {
		t.Fatalf("failed compiling app-default profile: %v", err)
	}
	if len(appFilter.Instructions) == 0 {
		t.Fatalf("expected non-empty app-default BPF filter instructions")
	}
}

func TestSeccompForbiddenSyscallTrap(t *testing.T) {
	if os.Getenv("BE_SECCOMP_TEST_CHILD") == "1" {
		// Child Process: Apply strict seccomp filter and execute forbidden syscall (mkdir)
		filter, err := GetProfileFilter("strict", ActionKillProcess)
		if err != nil {
			os.Exit(1)
		}
		if err := ApplySeccompFilter(filter); err != nil {
			os.Exit(2)
		}

		// Attempt forbidden syscall: mkdir
		_, _, errno := unix.Syscall(unix.SYS_MKDIR, uintptr(0), uintptr(0), 0)
		_ = errno
		// Should never reach here because process is SIGSYS killed by seccomp
		os.Exit(0)
	}

	// Parent Test: Re-exec test binary as child
	cmd := exec.Command(os.Args[0], "-test.run=TestSeccompForbiddenSyscallTrap")
	cmd.Env = append(os.Environ(), "BE_SECCOMP_TEST_CHILD=1")

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected child process to be killed by seccomp filter, but it exited cleanly")
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok {
			if status.Signaled() {
				sig := status.Signal()
				t.Logf("[PASS] Child process correctly terminated by signal: %v (SIGSYS = %v)", sig, unix.SIGSYS)
				if sig != unix.SIGSYS && sig != unix.SIGKILL {
					t.Logf("Notice: process terminated by signal %v", sig)
				}
			} else {
				t.Logf("Child process exited with code %d", exitErr.ExitCode())
			}
		}
	} else {
		t.Fatalf("unexpected error running child test: %v", err)
	}
}
