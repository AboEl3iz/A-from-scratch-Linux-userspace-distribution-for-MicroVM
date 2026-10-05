package secd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// SecurityPolicy defines the combined hardened isolation settings for a managed service.
type SecurityPolicy struct {
	SeccompProfile string   `json:"seccomp_profile"`
	CapsAdd        []string `json:"capabilities_add"`
	CapsDrop       []string `json:"capabilities_drop"`
	NoNewPrivs     bool     `json:"no_new_privs"`
	Rootfs         string   `json:"rootfs,omitempty"`
	MountNS        bool     `json:"mount_ns,omitempty"`
}

// DefaultSecurityPolicy returns a baseline hardened policy.
func DefaultSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		SeccompProfile: "app-default",
		CapsAdd:        nil,
		CapsDrop:       []string{"ALL"},
		NoNewPrivs:     true,
	}
}

// PivotRoot isolates the process mount namespace and pivots the root filesystem to newRoot.
func PivotRoot(newRoot string) error {
	if newRoot == "" {
		return nil
	}

	// 1. Unshare mount namespace so mounts don't propagate to host/parent namespace
	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return fmt.Errorf("secd: failed unsharing mount namespace: %w", err)
	}

	// 2. Set mount propagation on / to MS_PRIVATE
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("secd: failed setting MS_PRIVATE on /: %w", err)
	}

	// 3. Ensure newRoot is a mount point by bind mounting it onto itself if needed
	if err := unix.Mount(newRoot, newRoot, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("secd: bind mount failed on newRoot %s: %w", newRoot, err)
	}

	// 4. Create .oldroot inside newRoot
	putOld := filepath.Join(newRoot, ".oldroot")
	if err := os.MkdirAll(putOld, 0700); err != nil {
		return fmt.Errorf("secd: failed creating .oldroot in %s: %w", newRoot, err)
	}

	// 5. Pivot root
	if err := unix.PivotRoot(newRoot, putOld); err != nil {
		return fmt.Errorf("secd: pivot_root(%s, %s) failed: %w", newRoot, putOld, err)
	}

	// 6. Change directory to new root /
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("secd: chdir(/) after pivot_root failed: %w", err)
	}

	// 7. Mount essential pseudo-filesystems in new container rootfs
	_ = os.MkdirAll("/proc", 0755)
	_ = unix.Mount("proc", "/proc", "proc", 0, "")

	_ = os.MkdirAll("/sys", 0755)
	_ = unix.Mount("sysfs", "/sys", "sysfs", 0, "")

	_ = os.MkdirAll("/dev", 0755)
	_ = unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")

	// 8. Unmount old root and remove placeholder
	_ = unix.Unmount("/.oldroot", unix.MNT_DETACH)
	_ = os.Remove("/.oldroot")

	return nil
}

// EnforceSecurityPolicy applies PR_SET_NO_NEW_PRIVS, capability dropping, Seccomp BPF, and PivotRoot.
func EnforceSecurityPolicy(policy *SecurityPolicy) error {
	if policy == nil {
		return nil
	}

	// 0. PivotRoot & Mount Namespace isolation if Rootfs is specified
	if policy.Rootfs != "" {
		if err := PivotRoot(policy.Rootfs); err != nil {
			return fmt.Errorf("secd: pivot_root isolation failed: %w", err)
		}
	}

	// 1. PR_SET_NO_NEW_PRIVS
	if policy.NoNewPrivs {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("secd: failed setting PR_SET_NO_NEW_PRIVS: %w", err)
		}
	}

	// 2. Capability Dropping
	if len(policy.CapsDrop) > 0 || len(policy.CapsAdd) > 0 {
		_ = DropCapabilityBoundingSet(policy.CapsAdd)
		_ = DropCapabilities(policy.CapsAdd)
	}

	// 3. Seccomp BPF Syscall Filtering
	if policy.SeccompProfile != "" && policy.SeccompProfile != "unrestricted" {
		SetupSIGSYSHandler()
		filter, err := GetProfileFilter(policy.SeccompProfile, ActionKillProcess)
		if err != nil {
			return fmt.Errorf("secd: invalid seccomp profile %q: %w", policy.SeccompProfile, err)
		}
		if err := ApplySeccompFilter(filter); err != nil {
			return fmt.Errorf("secd: failed applying seccomp filter %q: %w", policy.SeccompProfile, err)
		}
	}

	return nil
}

// SetupSIGSYSHandler installs a signal channel listener for SIGSYS (Seccomp security violations).
func SetupSIGSYSHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGSYS)
	go func() {
		for sig := range c {
			fmt.Fprintf(os.Stderr, "[karim-secd | SECURITY WARNING] SIGSYS signal %v received: blocked by Seccomp BPF policy!\n", sig)
			os.Exit(159)
		}
	}()
}
