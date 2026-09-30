package system

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
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

const (
	FIFREEZE = 0xc0045877
	FITHAW   = 0xc0045878
)

// FreezeFilesystem flushes dirty pages and issues FIFREEZE ioctl on mountPath (default "/").
func FreezeFilesystem(mountPath string) error {
	if mountPath == "" {
		mountPath = "/"
	}
	syscall.Sync()

	f, err := os.Open(mountPath)
	if err != nil {
		return fmt.Errorf("failed opening mount %s for freeze: %w", mountPath, err)
	}
	defer f.Close()

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(FIFREEZE), 0)
	if errno != 0 && errno != syscall.EBUSY && errno != syscall.EINVAL && errno != syscall.ENOTTY {
		return fmt.Errorf("FIFREEZE ioctl failed on %s: %w", mountPath, errno)
	}
	return nil
}

// ThawFilesystem issues FITHAW ioctl on mountPath (default "/") to resume VFS transactions.
func ThawFilesystem(mountPath string) error {
	if mountPath == "" {
		mountPath = "/"
	}

	f, err := os.Open(mountPath)
	if err != nil {
		return fmt.Errorf("failed opening mount %s for thaw: %w", mountPath, err)
	}
	defer f.Close()

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(FITHAW), 0)
	if errno != 0 && errno != syscall.EINVAL && errno != syscall.ENOTTY {
		return fmt.Errorf("FITHAW ioctl failed on %s: %w", mountPath, errno)
	}
	return nil
}

// ReseedEntropy injects fresh host entropy into /dev/urandom on microVM snapshot restore.
func ReseedEntropy() error {
	hwrng, err := os.Open("/dev/hwrng")
	var randBuf []byte
	if err == nil {
		randBuf = make([]byte, 512)
		n, _ := hwrng.Read(randBuf)
		hwrng.Close()
		if n > 0 {
			randBuf = randBuf[:n]
		} else {
			randBuf = nil
		}
	}

	if len(randBuf) == 0 {
		randBuf = make([]byte, 64)
		nowNano := time.Now().UnixNano()
		pid := os.Getpid()
		for i := 0; i < len(randBuf); i++ {
			randBuf[i] = byte((nowNano >> (i % 8 * 8)) ^ int64(pid+i))
		}
	}

	urandom, err := os.OpenFile("/dev/urandom", os.O_WRONLY, 0)
	if err == nil {
		_, _ = urandom.Write(randBuf)
		urandom.Close()
	}
	return nil
}

// SyncRTCTime resynchronizes system clock CLOCK_REALTIME from hardware RTC device.
func SyncRTCTime() error {
	rtcFile, err := os.Open("/dev/rtc0")
	if err != nil {
		rtcFile, err = os.Open("/dev/rtc")
	}
	if err != nil {
		return nil
	}
	defer rtcFile.Close()

	var rtcTm struct {
		Sec, Min, Hour, Mday, Mon, Year, Wday, Yday, Isdst int32
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, rtcFile.Fd(), 0x80247009, uintptr(unsafe.Pointer(&rtcTm)))
	if errno != 0 {
		return fmt.Errorf("RTC_RD_TIME ioctl failed: %w", errno)
	}

	tm := time.Date(int(rtcTm.Year)+1900, time.Month(rtcTm.Mon+1), int(rtcTm.Mday), int(rtcTm.Hour), int(rtcTm.Min), int(rtcTm.Sec), 0, time.UTC)
	ts := unix.NsecToTimespec(tm.UnixNano())
	return unix.ClockSettime(unix.CLOCK_REALTIME, &ts)
}
