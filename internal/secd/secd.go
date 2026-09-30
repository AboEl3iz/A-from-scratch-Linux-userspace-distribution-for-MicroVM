package secd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// SecurityPolicy defines the combined hardened isolation settings for a managed service.
type SecurityPolicy struct {
	SeccompProfile string   `json:"seccomp_profile"`
	CapsAdd        []string `json:"capabilities_add"`
	CapsDrop       []string `json:"capabilities_drop"`
	NoNewPrivs     bool     `json:"no_new_privs"`
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

// EnforceSecurityPolicy applies PR_SET_NO_NEW_PRIVS, capability dropping, and Seccomp BPF filter to current process.
func EnforceSecurityPolicy(policy *SecurityPolicy) error {
	if policy == nil {
		return nil
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
