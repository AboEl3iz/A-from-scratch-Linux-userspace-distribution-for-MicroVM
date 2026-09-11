// +build ignore

#include "vmlinux.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} exec_events SEC(".maps");

SEC("tp/sched/sched_process_exec")
int handle_sched_process_exec(void *ctx)
{
    struct exec_event *event;
    __u64 pid_tgid;

    event = bpf_ringbuf_reserve(&exec_events, sizeof(*event), 0);
    if (!event)
        return 0;

    pid_tgid = bpf_get_current_pid_tgid();
    event->pid = (__u32)(pid_tgid >> 32);
    event->ppid = 0;

    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    // Tracepoint filename string copy
    bpf_probe_read_str(&event->filename, sizeof(event->filename), (void *)ctx);

    bpf_ringbuf_submit(event, 0);
    return 0;
}
