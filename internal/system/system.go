package system

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	procEntropyPath = "/proc/sys/kernel/random/entropy_avail"
	procCmdlinePath = "/proc/cmdline"
	rtcDevPath      = "/dev/rtc0"
)

// SystemStatus encapsulates guest system entropy, RTC time sync, and debug mode state.
type SystemStatus struct {
	EntropyAvail  int       `json:"entropy_avail"`
	RTCSynced     bool      `json:"rtc_synced"`
	CurrentTime   time.Time `json:"current_time"`
	DebugMode     bool      `json:"debug_mode"`
	KernelCmdline string    `json:"kernel_cmdline"`
	UptimeSeconds float64   `json:"uptime_seconds"`
}

// GetEntropyAvailable reads the current bits of available kernel entropy.
func GetEntropyAvailable() (int, error) {
	data, err := os.ReadFile(procEntropyPath)
	if err != nil {
		return 0, fmt.Errorf("failed reading %s: %w", procEntropyPath, err)
	}
	valStr := strings.TrimSpace(string(data))
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("invalid entropy value %q: %w", valStr, err)
	}
	return val, nil
}

// IsDebugEnabled checks if karim.debug=1 or karim.debug=shell was passed on kernel command line.
func IsDebugEnabled() bool {
	data, err := os.ReadFile(procCmdlinePath)
	if err != nil {
		return false
	}
	line := string(data)
	return strings.Contains(line, "karim.debug=1") || strings.Contains(line, "karim.debug=shell")
}

// ReadKernelCmdline returns the contents of /proc/cmdline.
func ReadKernelCmdline() string {
	data, err := os.ReadFile(procCmdlinePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// IsRTCAvailable checks if /dev/rtc0 or /dev/rtc device node is accessible.
func IsRTCAvailable() bool {
	_, err := os.Stat(rtcDevPath)
	if err == nil {
		return true
	}
	_, err = os.Stat("/dev/rtc")
	return err == nil
}

// GetUptimeSeconds returns system uptime in seconds using Sysinfo syscall.
func GetUptimeSeconds() float64 {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err == nil {
		return float64(info.Uptime)
	}
	return 0
}

// GetSystemStatus gathers comprehensive system health, entropy, and time metadata.
func GetSystemStatus() (*SystemStatus, error) {
	entropy, err := GetEntropyAvailable()
	if err != nil {
		entropy = -1
	}

	return &SystemStatus{
		EntropyAvail:  entropy,
		RTCSynced:     IsRTCAvailable(),
		CurrentTime:   time.Now().UTC(),
		DebugMode:     IsDebugEnabled(),
		KernelCmdline: ReadKernelCmdline(),
		UptimeSeconds: GetUptimeSeconds(),
	}, nil
}
