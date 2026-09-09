package secd

import (
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

// Standard Linux capability mappings
var capNameToNum = map[string]uintptr{
	"CAP_CHOWN":              unix.CAP_CHOWN,
	"CAP_DAC_OVERRIDE":       unix.CAP_DAC_OVERRIDE,
	"CAP_DAC_READ_SEARCH":    unix.CAP_DAC_READ_SEARCH,
	"CAP_FOWNER":             unix.CAP_FOWNER,
	"CAP_FSETID":             unix.CAP_FSETID,
	"CAP_KILL":               unix.CAP_KILL,
	"CAP_SETGID":             unix.CAP_SETGID,
	"CAP_SETUID":             unix.CAP_SETUID,
	"CAP_SETPCAP":            unix.CAP_SETPCAP,
	"CAP_LINUX_IMMUTABLE":    unix.CAP_LINUX_IMMUTABLE,
	"CAP_NET_BIND_SERVICE":   unix.CAP_NET_BIND_SERVICE,
	"CAP_NET_BROADCAST":      unix.CAP_NET_BROADCAST,
	"CAP_NET_ADMIN":          unix.CAP_NET_ADMIN,
	"CAP_NET_RAW":            unix.CAP_NET_RAW,
	"CAP_IPC_LOCK":           unix.CAP_IPC_LOCK,
	"CAP_IPC_OWNER":          unix.CAP_IPC_OWNER,
	"CAP_SYS_MODULE":         unix.CAP_SYS_MODULE,
	"CAP_SYS_RAWIO":          unix.CAP_SYS_RAWIO,
	"CAP_SYS_CHROOT":         unix.CAP_SYS_CHROOT,
	"CAP_SYS_PTRACE":         unix.CAP_SYS_PTRACE,
	"CAP_SYS_PACCT":          unix.CAP_SYS_PACCT,
	"CAP_SYS_ADMIN":          unix.CAP_SYS_ADMIN,
	"CAP_SYS_BOOT":           unix.CAP_SYS_BOOT,
	"CAP_SYS_NICE":           unix.CAP_SYS_NICE,
	"CAP_SYS_RESOURCE":       unix.CAP_SYS_RESOURCE,
	"CAP_SYS_TIME":           unix.CAP_SYS_TIME,
	"CAP_SYS_TTY_CONFIG":     unix.CAP_SYS_TTY_CONFIG,
	"CAP_MKNOD":              unix.CAP_MKNOD,
	"CAP_LEASE":              unix.CAP_LEASE,
	"CAP_AUDIT_WRITE":        unix.CAP_AUDIT_WRITE,
	"CAP_AUDIT_CONTROL":      unix.CAP_AUDIT_CONTROL,
	"CAP_SETFCAP":            unix.CAP_SETFCAP,
	"CAP_MAC_OVERRIDE":       unix.CAP_MAC_OVERRIDE,
	"CAP_MAC_ADMIN":          unix.CAP_MAC_ADMIN,
	"CAP_SYSLOG":             unix.CAP_SYSLOG,
	"CAP_WAKE_ALARM":         unix.CAP_WAKE_ALARM,
	"CAP_BLOCK_SUSPEND":      unix.CAP_BLOCK_SUSPEND,
	"CAP_AUDIT_READ":         unix.CAP_AUDIT_READ,
	"CAP_PERFMON":            unix.CAP_PERFMON,
	"CAP_BPF":                unix.CAP_BPF,
	"CAP_CHECKPOINT_RESTORE": unix.CAP_CHECKPOINT_RESTORE,
}

// ParseCapability converts string (e.g. "CAP_NET_BIND_SERVICE" or "NET_BIND_SERVICE") to numeric capability value.
func ParseCapability(name string) (uintptr, error) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if !strings.HasPrefix(upper, "CAP_") {
		upper = "CAP_" + upper
	}
	if capNum, ok := capNameToNum[upper]; ok {
		return capNum, nil
	}
	return 0, fmt.Errorf("unknown capability: %q", name)
}

// DropCapabilities configures effective, permitted, and inheritable capability sets based on keepCaps allowlist.
// If keepCaps contains "ALL" (case insensitive), no capabilities are dropped.
// If keepCaps is empty or contains "NONE", all capabilities are dropped.
func DropCapabilities(keepCaps []string) error {
	for _, capStr := range keepCaps {
		if strings.EqualFold(strings.TrimSpace(capStr), "ALL") {
			return nil
		}
	}

	var effLow, effHigh uint32
	var permLow, permHigh uint32

	for _, capStr := range keepCaps {
		if strings.EqualFold(strings.TrimSpace(capStr), "NONE") {
			continue
		}
		capNum, err := ParseCapability(capStr)
		if err != nil {
			return err
		}
		if capNum < 32 {
			effLow |= (1 << capNum)
			permLow |= (1 << capNum)
		} else {
			effHigh |= (1 << (capNum - 32))
			permHigh |= (1 << (capNum - 32))
		}
	}

	header := unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0, // Current process
	}
	data := [2]unix.CapUserData{
		{
			Effective:   effLow,
			Permitted:   permLow,
			Inheritable: 0,
		},
		{
			Effective:   effHigh,
			Permitted:   permHigh,
			Inheritable: 0,
		},
	}

	if err := unix.Capset(&header, &data[0]); err != nil {
		return fmt.Errorf("capset failed: %w", err)
	}
	return nil
}

// DropAllCapabilities removes all capabilities from effective, permitted, and inheritable sets.
func DropAllCapabilities() error {
	return DropCapabilities([]string{"NONE"})
}

// DropCapabilityBoundingSet drops all capabilities from the bounding set except keepCaps.
func DropCapabilityBoundingSet(keepCaps []string) error {
	keepMap := make(map[uintptr]bool)
	for _, capStr := range keepCaps {
		if strings.EqualFold(strings.TrimSpace(capStr), "ALL") {
			return nil
		}
		if strings.EqualFold(strings.TrimSpace(capStr), "NONE") {
			continue
		}
		c, err := ParseCapability(capStr)
		if err == nil {
			keepMap[c] = true
		}
	}

	for c := uintptr(0); c <= 63; c++ {
		if keepMap[c] {
			continue
		}
		_ = unix.Prctl(unix.PR_CAPBSET_DROP, c, 0, 0, 0)
	}
	return nil
}
