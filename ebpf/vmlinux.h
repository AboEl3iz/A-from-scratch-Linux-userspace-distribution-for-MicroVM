#ifndef __VMLINUX_H__
#define __VMLINUX_H__

typedef unsigned char __u8;
typedef short int __s16;
typedef unsigned short __u16;
typedef int __s32;
typedef unsigned int __u32;
typedef long long int __s64;
typedef unsigned long long __u64;

typedef __u16 __sum16;
typedef __u32 __wsum;

typedef __u32 pid_t;
typedef __u32 dev_t;

#define TASK_COMM_LEN 16
#define MAX_FILENAME_LEN 256
#define MAX_SLOTS 20

// Event structure emitted by execsnoop probe over ringbuffer
struct exec_event {
    __u32 pid;
    __u32 ppid;
    char comm[TASK_COMM_LEN];
    char filename[MAX_FILENAME_LEN];
};

// Histograms bucket array format (20 log2 slots)
struct hist {
    __u64 slots[MAX_SLOTS];
};

// Sched wakeup / switch tracepoint argument structures
struct trace_event_raw_sched_wakeup {
    __u64 pad;
    char comm[TASK_COMM_LEN];
    pid_t pid;
    int prio;
    int success;
    int target_cpu;
};

struct trace_event_raw_sched_switch {
    __u64 pad;
    char prev_comm[TASK_COMM_LEN];
    pid_t prev_pid;
    int prev_prio;
    long long prev_state;
    char next_comm[TASK_COMM_LEN];
    pid_t next_pid;
    int next_prio;
};

// Block I/O tracepoint argument structures
struct trace_event_raw_block_rq_issue {
    __u64 pad;
    dev_t dev;
    unsigned long long sector;
    unsigned int nr_sector;
    char bytes[8];
    char rwbs[8];
    char comm[TASK_COMM_LEN];
    char cmd[4];
};

struct trace_event_raw_block_rq_complete {
    __u64 pad;
    dev_t dev;
    unsigned long long sector;
    unsigned int nr_sector;
    int error;
    char rwbs[8];
};

#endif /* __VMLINUX_H__ */
