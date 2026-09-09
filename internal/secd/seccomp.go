package secd

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// BPF instruction codes
const (
	bpfLdW   = unix.BPF_LD | unix.BPF_W | unix.BPF_ABS
	bpfJmpEq = unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K
	bpfRetK  = unix.BPF_RET | unix.BPF_K
)

// Common x86_64 Syscall Numbers for Profile Building
var SyscallMap = map[string]uint32{
	"read":                   unix.SYS_READ,
	"write":                  unix.SYS_WRITE,
	"open":                   unix.SYS_OPEN,
	"close":                  unix.SYS_CLOSE,
	"stat":                   unix.SYS_STAT,
	"fstat":                  unix.SYS_FSTAT,
	"lstat":                  unix.SYS_LSTAT,
	"poll":                   unix.SYS_POLL,
	"lseek":                  unix.SYS_LSEEK,
	"mmap":                   unix.SYS_MMAP,
	"mprotect":               unix.SYS_MPROTECT,
	"munmap":                 unix.SYS_MUNMAP,
	"brk":                    unix.SYS_BRK,
	"rt_sigaction":           unix.SYS_RT_SIGACTION,
	"rt_sigprocmask":         unix.SYS_RT_SIGPROCMASK,
	"rt_sigreturn":           unix.SYS_RT_SIGRETURN,
	"ioctl":                  unix.SYS_IOCTL,
	"pread64":                unix.SYS_PREAD64,
	"pwrite64":               unix.SYS_PWRITE64,
	"readv":                  unix.SYS_READV,
	"writev":                 unix.SYS_WRITEV,
	"access":                 unix.SYS_ACCESS,
	"pipe":                   unix.SYS_PIPE,
	"select":                 unix.SYS_SELECT,
	"sched_yield":            unix.SYS_SCHED_YIELD,
	"madvise":                unix.SYS_MADVISE,
	"dup":                    unix.SYS_DUP,
	"dup2":                   unix.SYS_DUP2,
	"getpid":                 unix.SYS_GETPID,
	"pause":                  unix.SYS_PAUSE,
	"nanosleep":              unix.SYS_NANOSLEEP,
	"getitimer":              unix.SYS_GETITIMER,
	"alarm":                  unix.SYS_ALARM,
	"setitimer":              unix.SYS_SETITIMER,
	"getppid":                unix.SYS_GETPPID,
	"getpgrp":                unix.SYS_GETPGRP,
	"setsid":                 unix.SYS_SETSID,
	"getuid":                 unix.SYS_GETUID,
	"geteuid":                unix.SYS_GETEUID,
	"getgid":                 unix.SYS_GETGID,
	"getegid":                unix.SYS_GETEGID,
	"arch_prctl":             unix.SYS_ARCH_PRCTL,
	"gettid":                 unix.SYS_GETTID,
	"futex":                  unix.SYS_FUTEX,
	"sched_getaffinity":      unix.SYS_SCHED_GETAFFINITY,
	"exit":                   unix.SYS_EXIT,
	"exit_group":             unix.SYS_EXIT_GROUP,
	"epoll_create":           unix.SYS_EPOLL_CREATE,
	"epoll_create1":          unix.SYS_EPOLL_CREATE1,
	"epoll_ctl":              unix.SYS_EPOLL_CTL,
	"epoll_wait":             unix.SYS_EPOLL_WAIT,
	"epoll_pwait":            unix.SYS_EPOLL_PWAIT,
	"eventfd":                unix.SYS_EVENTFD,
	"eventfd2":               unix.SYS_EVENTFD2,
	"timerfd_create":         unix.SYS_TIMERFD_CREATE,
	"timerfd_settime":        unix.SYS_TIMERFD_SETTIME,
	"timerfd_gettime":        unix.SYS_TIMERFD_GETTIME,
	"clock_gettime":          unix.SYS_CLOCK_GETTIME,
	"clock_getres":           unix.SYS_CLOCK_GETRES,
	"clock_nanosleep":        unix.SYS_CLOCK_NANOSLEEP,
	"fcntl":                  unix.SYS_FCNTL,
	"getcwd":                 unix.SYS_GETCWD,
	"chdir":                  unix.SYS_CHDIR,
	"fchdir":                 unix.SYS_FCHDIR,
	"mkdir":                  unix.SYS_MKDIR,
	"rmdir":                  unix.SYS_RMDIR,
	"unlink":                 unix.SYS_UNLINK,
	"readlink":               unix.SYS_READLINK,
	"openat":                 unix.SYS_OPENAT,
	"mkdirat":                unix.SYS_MKDIRAT,
	"unlinkat":               unix.SYS_UNLINKAT,
	"readlinkat":             unix.SYS_READLINKAT,
	"newfstatat":             unix.SYS_NEWFSTATAT,
	"pipe2":                  unix.SYS_PIPE2,
	"dup3":                   unix.SYS_DUP3,
	"prctl":                  unix.SYS_PRCTL,
	"sysinfo":                unix.SYS_SYSINFO,
	"uname":                  unix.SYS_UNAME,
	"tgkill":                 unix.SYS_TGKILL,
	"execve":                 unix.SYS_EXECVE,
	"execveat":               unix.SYS_EXECVEAT,
	"wait4":                  unix.SYS_WAIT4,
	"clone":                  unix.SYS_CLONE,
	"clone3":                 unix.SYS_CLONE3,
	"sigaltstack":            unix.SYS_SIGALTSTACK,
	"set_robust_list":        unix.SYS_SET_ROBUST_LIST,
	"get_robust_list":        unix.SYS_GET_ROBUST_LIST,
	"set_tid_address":        unix.SYS_SET_TID_ADDRESS,
	"set_thread_area":         unix.SYS_SET_THREAD_AREA,
	"gettimeofday":           unix.SYS_GETTIMEOFDAY,
	"rseq":                   unix.SYS_RSEQ,
	"prlimit64":              unix.SYS_PRLIMIT64,
	"getrandom":              unix.SYS_GETRANDOM,

	// Networking
	"socket":                 unix.SYS_SOCKET,
	"connect":                unix.SYS_CONNECT,
	"accept":                 unix.SYS_ACCEPT,
	"sendto":                 unix.SYS_SENDTO,
	"recvfrom":               unix.SYS_RECVFROM,
	"sendmsg":                unix.SYS_SENDMSG,
	"recvmsg":                unix.SYS_RECVMSG,
	"shutdown":               unix.SYS_SHUTDOWN,
	"bind":                   unix.SYS_BIND,
	"listen":                 unix.SYS_LISTEN,
	"getsockname":            unix.SYS_GETSOCKNAME,
	"getpeername":            unix.SYS_GETPEERNAME,
	"socketpair":             unix.SYS_SOCKETPAIR,
	"setsockopt":             unix.SYS_SETSOCKOPT,
	"getsockopt":             unix.SYS_GETSOCKOPT,
	"accept4":                unix.SYS_ACCEPT4,

	// Forbidden / Admin Syscalls
	"reboot":                 unix.SYS_REBOOT,
	"kexec_load":             unix.SYS_KEXEC_LOAD,
	"init_module":            unix.SYS_INIT_MODULE,
	"delete_module":          unix.SYS_DELETE_MODULE,
	"swapon":                 unix.SYS_SWAPON,
	"swapoff":                unix.SYS_SWAPOFF,
	"pivot_root":             unix.SYS_PIVOT_ROOT,
	"ptrace":                 unix.SYS_PTRACE,
}

// Resolution modes for Seccomp BPF filters
type SeccompAction uint32

const (
	ActionAllow       SeccompAction = unix.SECCOMP_RET_ALLOW
	ActionKillProcess SeccompAction = unix.SECCOMP_RET_KILL_PROCESS
	ActionTrap        SeccompAction = unix.SECCOMP_RET_TRAP
	ActionErrno       SeccompAction = unix.SECCOMP_RET_ERRNO | 1 // EPERM
)

// SeccompFilter represents a compiled BPF program
type SeccompFilter struct {
	Instructions []unix.SockFilter
}

// CompileWhitelistFilter generates a BPF sock_filter array allowing specified syscalls, defaulting to defaultAction.
func CompileWhitelistFilter(allowedSyscalls []uint32, defaultAction SeccompAction) *SeccompFilter {
	filter := []unix.SockFilter{
		// 0: Load architecture (seccomp_data.arch offset 4)
		{Code: bpfLdW, Jt: 0, Jf: 0, K: 4},
		// 1: Verify arch == AUDIT_ARCH_X86_64
		{Code: bpfJmpEq, Jt: 1, Jf: 0, K: unix.AUDIT_ARCH_X86_64},
		// 2: Arch mismatch -> kill process
		{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(ActionKillProcess)},
		// 3: Load syscall number (seccomp_data.nr offset 0)
		{Code: bpfLdW, Jt: 0, Jf: 0, K: 0},
	}

	n := len(allowedSyscalls)
	for i, sysNum := range allowedSyscalls {
		// Jump if syscall == sysNum
		// Jt = steps to skip trailing checks + default action = (n - i)
		filter = append(filter, unix.SockFilter{
			Code: bpfJmpEq,
			Jt:   uint8(n - i),
			Jf:   0,
			K:    sysNum,
		})
	}

	// Default action if no match
	filter = append(filter, unix.SockFilter{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(defaultAction)})

	// Allowed action
	filter = append(filter, unix.SockFilter{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(ActionAllow)})

	return &SeccompFilter{Instructions: filter}
}

// CompileBlacklistFilter generates a BPF sock_filter array prohibiting specified syscalls with deniedAction, defaulting to ALLOW.
func CompileBlacklistFilter(deniedSyscalls []uint32, deniedAction SeccompAction) *SeccompFilter {
	filter := []unix.SockFilter{
		// 0: Load arch
		{Code: bpfLdW, Jt: 0, Jf: 0, K: 4},
		// 1: Check AUDIT_ARCH_X86_64
		{Code: bpfJmpEq, Jt: 1, Jf: 0, K: unix.AUDIT_ARCH_X86_64},
		// 2: Arch mismatch -> kill
		{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(ActionKillProcess)},
		// 3: Load syscall nr
		{Code: bpfLdW, Jt: 0, Jf: 0, K: 0},
	}

	n := len(deniedSyscalls)
	for i, sysNum := range deniedSyscalls {
		filter = append(filter, unix.SockFilter{
			Code: bpfJmpEq,
			Jt:   uint8(n - i),
			Jf:   0,
			K:    sysNum,
		})
	}

	// Default action if no blacklist match -> ALLOW
	filter = append(filter, unix.SockFilter{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(ActionAllow)})

	// Denied action if matched
	filter = append(filter, unix.SockFilter{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(deniedAction)})

	return &SeccompFilter{Instructions: filter}
}

// GetProfileFilter retrieves or compiles a named seccomp profile filter.
func GetProfileFilter(profileName string, defaultAction SeccompAction) (*SeccompFilter, error) {
	name := strings.ToLower(strings.TrimSpace(profileName))
	switch name {
	case "unrestricted", "disabled", "none", "":
		// Allow everything
		filter := []unix.SockFilter{
			{Code: bpfRetK, Jt: 0, Jf: 0, K: uint32(ActionAllow)},
		}
		return &SeccompFilter{Instructions: filter}, nil

	case "strict":
		allowedNames := []string{
			"read", "write", "close", "fstat", "mmap", "mprotect", "munmap",
			"brk", "rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "exit",
			"exit_group", "futex", "arch_prctl", "getrandom", "nanosleep",
			"set_tid_address", "set_thread_area", "gettimeofday",
		}
		return compileFromNames(allowedNames, defaultAction)

	case "app-default":
		allowedNames := []string{
			"read", "write", "open", "close", "stat", "fstat", "lstat", "poll", "lseek",
			"mmap", "mprotect", "munmap", "brk", "rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
			"ioctl", "pread64", "pwrite64", "readv", "writev", "access", "pipe", "select",
			"sched_yield", "madvise", "dup", "dup2", "getpid", "pause", "nanosleep",
			"getppid", "getpgrp", "setsid", "getuid", "geteuid", "getgid", "getegid",
			"arch_prctl", "gettid", "futex", "sched_getaffinity", "exit", "exit_group",
			"epoll_create", "epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait",
			"eventfd", "eventfd2", "timerfd_create", "timerfd_settime", "timerfd_gettime",
			"clock_gettime", "clock_getres", "clock_nanosleep", "fcntl", "getcwd", "chdir",
			"fchdir", "mkdir", "rmdir", "unlink", "readlink", "openat", "mkdirat", "unlinkat",
			"readlinkat", "newfstatat", "pipe2", "dup3", "prctl", "sysinfo", "uname", "tgkill",
			"execve", "execveat", "wait4", "clone", "clone3", "sigaltstack", "set_robust_list",
			"get_robust_list", "rseq", "prlimit64", "getrandom",
			"set_tid_address", "set_thread_area", "gettimeofday",
		}
		return compileFromNames(allowedNames, defaultAction)

	case "net-service":
		allowedNames := []string{
			"read", "write", "open", "close", "stat", "fstat", "lstat", "poll", "lseek",
			"mmap", "mprotect", "munmap", "brk", "rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
			"ioctl", "pread64", "pwrite64", "readv", "writev", "access", "pipe", "select",
			"sched_yield", "madvise", "dup", "dup2", "getpid", "pause", "nanosleep",
			"getppid", "getpgrp", "setsid", "getuid", "geteuid", "getgid", "getegid",
			"arch_prctl", "gettid", "futex", "sched_getaffinity", "exit", "exit_group",
			"epoll_create", "epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait",
			"eventfd", "eventfd2", "timerfd_create", "timerfd_settime", "timerfd_gettime",
			"clock_gettime", "clock_getres", "clock_nanosleep", "fcntl", "getcwd", "chdir",
			"fchdir", "mkdir", "rmdir", "unlink", "readlink", "openat", "mkdirat", "unlinkat",
			"readlinkat", "newfstatat", "pipe2", "dup3", "prctl", "sysinfo", "uname", "tgkill",
			"execve", "execveat", "wait4", "clone", "clone3", "sigaltstack", "set_robust_list",
			"get_robust_list", "rseq", "prlimit64", "getrandom",
			"set_tid_address", "set_thread_area", "gettimeofday",
			// Network additions
			"socket", "connect", "accept", "sendto", "recvfrom", "sendmsg", "recvmsg",
			"shutdown", "bind", "listen", "getsockname", "getpeername", "socketpair",
			"setsockopt", "getsockopt", "accept4",
		}
		return compileFromNames(allowedNames, defaultAction)

	default:
		return nil, fmt.Errorf("unknown seccomp profile: %q", profileName)
	}
}

func compileFromNames(names []string, defaultAction SeccompAction) (*SeccompFilter, error) {
	var nums []uint32
	seen := make(map[uint32]bool)
	for _, n := range names {
		if sysNum, ok := SyscallMap[strings.ToLower(strings.TrimSpace(n))]; ok {
			if !seen[sysNum] {
				seen[sysNum] = true
				nums = append(nums, sysNum)
			}
		}
	}
	return CompileWhitelistFilter(nums, defaultAction), nil
}

// ApplySeccompFilter loads the compiled filter into the calling process kernel seccomp engine.
func ApplySeccompFilter(filter *SeccompFilter) error {
	if filter == nil || len(filter.Instructions) == 0 {
		return nil
	}

	// 1. Ensure PR_SET_NO_NEW_PRIVS is active
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("failed setting PR_SET_NO_NEW_PRIVS: %w", err)
	}

	prog := unix.SockFprog{
		Len:    uint16(len(filter.Instructions)),
		Filter: &filter.Instructions[0],
	}

	// 2. Load filter via prctl
	if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(&prog)), 0, 0); err != nil {
		return fmt.Errorf("failed setting SECCOMP_MODE_FILTER: %w", err)
	}

	return nil
}
