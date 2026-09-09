/*
 * Karim MicroVM OS - PID 1 Static C Init (karim-init)
 * 
 * Responsibilities:
 * 1. Mount virtual pseudo-filesystems (/proc, /sys, /dev, /sys/fs/cgroup)
 * 2. Configure early serial console log redirection
 * 3. Handle SIGCHLD signals and reap zombie processes asynchronously via waitpid()
 * 4. Verify zombie process reaping with a test child process
 * 5. Hand off control to the secondary Go supervisor daemon (/sbin/karim-svcd)
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <signal.h>
#include <string.h>
#include <errno.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>

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
    test_zombie_reaper();

    log_info("Attempting handoff to secondary supervisor (/sbin/karim-svcd)...");
    
    char *svcd_argv[] = { "/sbin/karim-svcd", NULL };
    execve("/sbin/karim-svcd", svcd_argv, envp);

    /* If execve fails (e.g. Phase 0 before rootfs is attached), fall back to monitoring loop */
    log_info("Notice: /sbin/karim-svcd not found or failed to execute. Fallback init loop active.");

    while (1) {
        /* Reap any remaining zombies */
        int status;
        while (waitpid(-1, &status, WNOHANG) > 0);
        pause();
    }

    return 0;
}
