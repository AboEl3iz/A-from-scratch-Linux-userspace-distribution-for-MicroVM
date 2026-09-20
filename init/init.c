/*
 * Karim MicroVM OS - PID 1 Static C Init (karim-init)
 * 
 * Responsibilities:
 * 1. Mount virtual pseudo-filesystems (/proc, /sys, /dev, /sys/fs/cgroup)
 * 2. Configure early serial console log redirection
 * 3. Handle SIGCHLD signals and reap zombie processes asynchronously via waitpid()
 * 4. Verify zombie process reaping with a test child process
 * 5. Initialize hardware entropy (virtio-rng -> /dev/urandom)
 * 6. Synchronize Real-Time Clock (/dev/rtc0 -> CLOCK_REALTIME)
 * 7. Parse kernel command line (/proc/cmdline) for debug flags (karim.debug=1)
 * 8. Hand off control to secondary Go supervisor daemon (/sbin/karim-svcd)
 * 9. Fallback to emergency interactive debug shell if requested or on handoff failure
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <signal.h>
#include <string.h>
#include <errno.h>
#include <time.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/ioctl.h>
struct rtc_time {
    int tm_sec;
    int tm_min;
    int tm_hour;
    int tm_mday;
    int tm_mon;
    int tm_year;
    int tm_wday;
    int tm_yday;
    int tm_isdst;
};

#ifndef RTC_RD_TIME
#define RTC_RD_TIME _IOR('p', 0x09, struct rtc_time)
#endif


#define LOG_PREFIX "[karim-init] "

static void log_info(const char *msg) {
    printf("%s%s\n", LOG_PREFIX, msg);
    fflush(stdout);
}

static void log_error(const char *msg) {
    fprintf(stderr, "%sERROR: %s (errno: %s)\n", LOG_PREFIX, msg, strerror(errno));
    fflush(stderr);
}

/* Signal handler for SIGCHLD to reap defunct/zombie processes */
static void sigchld_handler(int sig) {
    (void)sig;
    int saved_errno = errno;
    pid_t pid;
    int status;

    while ((pid = waitpid(-1, &status, WNOHANG)) > 0) {
        if (WIFEXITED(status)) {
            printf("%sReaped child PID %d (exit code %d)\n", LOG_PREFIX, pid, WEXITSTATUS(status));
        } else if (WIFSIGNALED(status)) {
            printf("%sReaped child PID %d (killed by signal %d)\n", LOG_PREFIX, pid, WTERMSIG(status));
        }
        fflush(stdout);
    }
    errno = saved_errno;
}

static void setup_signals(void) {
    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = sigchld_handler;
    sa.sa_flags = SA_NOCLDSTOP | SA_RESTART;
    sigemptyset(&sa.sa_mask);

    if (sigaction(SIGCHLD, &sa, NULL) < 0) {
        log_error("Failed to install SIGCHLD handler");
    }
}

static void mount_pseudofs(void) {
    log_info("Mounting pseudo-filesystems (/proc, /sys, /dev, /sys/fs/cgroup)...");

    mkdir("/proc", 0755);
    if (mount("proc", "/proc", "proc", MS_NOSUID | MS_NOEXEC | MS_NODEV, NULL) < 0) {
        if (errno != EBUSY) log_error("Failed to mount /proc");
    }

    mkdir("/sys", 0755);
    if (mount("sysfs", "/sys", "sysfs", MS_NOSUID | MS_NOEXEC | MS_NODEV, NULL) < 0) {
        if (errno != EBUSY) log_error("Failed to mount /sys");
    }

    mkdir("/dev", 0755);
    if (mount("devtmpfs", "/dev", "devtmpfs", MS_NOSUID, NULL) < 0) {
        if (errno != EBUSY) log_error("Failed to mount /dev");
    }

    mkdir("/sys/fs/cgroup", 0755);
    if (mount("cgroup2", "/sys/fs/cgroup", "cgroup2", MS_NOSUID | MS_NOEXEC | MS_NODEV, NULL) < 0) {
        if (errno != EBUSY) log_error("Failed to mount /sys/fs/cgroup");
    }
}

static int parse_kernel_cmdline(void) {
    int debug_mode = 0;
    int fd = open("/proc/cmdline", O_RDONLY);
    if (fd < 0) return 0;

    char buf[1024];
    ssize_t n = read(fd, buf, sizeof(buf) - 1);
    close(fd);

    if (n > 0) {
        buf[n] = '\0';
        if (strstr(buf, "karim.debug=1") != NULL || strstr(buf, "karim.debug=shell") != NULL) {
            debug_mode = 1;
        }
    }
    return debug_mode;
}

static void init_entropy_pool(void) {
    log_info("Initializing hardware entropy pool (virtio-rng / /dev/hwrng)...");
    
    int entropy_avail = 0;
    FILE *f = fopen("/proc/sys/kernel/random/entropy_avail", "r");
    if (f) {
        if (fscanf(f, "%d", &entropy_avail) != 1) entropy_avail = 0;
        fclose(f);
    }

    int hwrng_fd = open("/dev/hwrng", O_RDONLY | O_NONBLOCK);
    if (hwrng_fd >= 0) {
        int urandom_fd = open("/dev/urandom", O_WRONLY);
        if (urandom_fd >= 0) {
            char rand_buf[512];
            ssize_t r = read(hwrng_fd, rand_buf, sizeof(rand_buf));
            if (r > 0) {
                ssize_t w = write(urandom_fd, rand_buf, r);
                (void)w;
                log_info("Seeded /dev/urandom with bytes from virtio-rng /dev/hwrng.");
            }
            close(urandom_fd);
        }
        close(hwrng_fd);
    } else {
        log_info("Notice: /dev/hwrng not available; relying on kernel CPU random source.");
    }

    f = fopen("/proc/sys/kernel/random/entropy_avail", "r");
    if (f) {
        if (fscanf(f, "%d", &entropy_avail) != 1) entropy_avail = 0;
        fclose(f);
    }
    printf("%sKernel entropy available: %d bits\n", LOG_PREFIX, entropy_avail);
    fflush(stdout);
}

static void sync_rtc_time(void) {
    log_info("Synchronizing hardware Real-Time Clock (/dev/rtc0)...");

    int rtc_fd = open("/dev/rtc0", O_RDONLY);
    if (rtc_fd < 0) {
        rtc_fd = open("/dev/rtc", O_RDONLY);
    }

    if (rtc_fd < 0) {
        log_info("Notice: /dev/rtc0 hardware clock device not present.");
        return;
    }

    struct rtc_time rtc_tm;
    memset(&rtc_tm, 0, sizeof(rtc_tm));
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Woverflow"
    if (ioctl(rtc_fd, RTC_RD_TIME, &rtc_tm) < 0) {
#pragma GCC diagnostic pop

        log_error("Failed to read hardware RTC time via ioctl");
        close(rtc_fd);
        return;
    }
    close(rtc_fd);

    struct tm tm;
    memset(&tm, 0, sizeof(tm));
    tm.tm_sec = rtc_tm.tm_sec;
    tm.tm_min = rtc_tm.tm_min;
    tm.tm_hour = rtc_tm.tm_hour;
    tm.tm_mday = rtc_tm.tm_mday;
    tm.tm_mon = rtc_tm.tm_mon;
    tm.tm_year = rtc_tm.tm_year;
    tm.tm_isdst = -1;

    time_t rtc_sec = timegm(&tm);
    if (rtc_sec != (time_t)-1) {
        struct timespec ts;
        ts.tv_sec = rtc_sec;
        ts.tv_nsec = 0;
        if (clock_settime(CLOCK_REALTIME, &ts) == 0) {
            char time_buf[64];
            strftime(time_buf, sizeof(time_buf), "%Y-%m-%d %H:%M:%S UTC", &tm);
            printf("%sRTC time synced successfully: %s\n", LOG_PREFIX, time_buf);
            fflush(stdout);
            return;
        }
    }
    log_error("Failed setting system clock from RTC time");
}

static void spawn_debug_shell(void) {
    log_info("======================================================================");
    log_info("        Karim MicroVM OS — Emergency Interactive Debug Shell          ");
    log_info("======================================================================");

    if (access("/bin/sh", X_OK) == 0) {
        pid_t pid = fork();
        if (pid == 0) {
            char *sh_argv[] = { "/bin/sh", "-i", NULL };
            execve("/bin/sh", sh_argv, NULL);
            _exit(1);
        } else if (pid > 0) {
            int status;
            waitpid(pid, &status, 0);
            log_info("Emergency debug shell session ended.");
        }
    } else {
        log_info("Notice: /bin/sh binary not found for emergency debug shell.");
    }
}

static void test_zombie_reaper(void) {
    log_info("Spawning test child process to verify zombie process reaper...");
    pid_t pid = fork();
    if (pid < 0) {
        log_error("Failed to fork test child process");
    } else if (pid == 0) {
        /* Child process exits immediately */
        _exit(42);
    } else {
        /* Parent sleeps briefly to allow SIGCHLD signal handler to execute */
        usleep(50000);
    }
}

int main(int argc, char *argv[], char *envp[]) {
    (void)argc;
    (void)argv;

    /* Setup early log redirection to /dev/ttyS0 or console */
    int console_fd = open("/dev/ttyS0", O_RDWR | O_NONBLOCK);
    if (console_fd < 0) {
        console_fd = open("/dev/console", O_RDWR | O_NONBLOCK);
    }
    if (console_fd >= 0) {
        dup2(console_fd, STDOUT_FILENO);
        dup2(console_fd, STDERR_FILENO);
        if (console_fd > STDERR_FILENO) close(console_fd);
    }

    log_info("Karim MicroVM OS - PID 1 Init starting...");
    
    setup_signals();
    mount_pseudofs();
    
    int debug_mode = parse_kernel_cmdline();
    if (debug_mode) {
        log_info("Kernel command line debug flag detected (karim.debug=1).");
    }

    init_entropy_pool();
    sync_rtc_time();
    test_zombie_reaper();

    if (debug_mode) {
        spawn_debug_shell();
    }

    log_info("Attempting handoff to secondary supervisor (/sbin/karim-svcd)...");
    
    char *svcd_argv[] = { "/sbin/karim-svcd", NULL };
    execve("/sbin/karim-svcd", svcd_argv, envp);

    /* If execve fails (e.g. Phase 0 before rootfs is attached), fall back to monitoring loop */
    log_info("Notice: /sbin/karim-svcd not found or failed to execute. Fallback init loop active.");

    if (!debug_mode) {
        spawn_debug_shell();
    }

    while (1) {
        /* Reap any remaining zombies */
        int status;
        while (waitpid(-1, &status, WNOHANG) > 0);
        pause();
    }

    return 0;
}

