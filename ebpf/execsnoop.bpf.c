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

SEC("tp_btf/sched_process_exec")
int BPF_PROG(handle_sched_process_exec, struct task_struct *p, pid_t old_pid, struct linux_binprm *bprm)
{
    struct exec_event *event;
    __u64 pid_tgid;

    event = bpf_ringbuf_reserve(&exec_events, sizeof(*event), 0);
    if (!event)
        return 0;

    pid_tgid = bpf_get_current_pid_tgid();
    event->pid = (__u32)(pid_tgid >> 32);
    event->ppid = p ? (p->real_parent ? p->real_parent->pid : 0) : 0;

    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    if (bprm && bprm->filename) {
        bpf_probe_read_str(&event->filename, sizeof(event->filename), bprm->filename);
    } else {
        event->filename[0] = '\0';
    }

    bpf_ringbuf_submit(event, 0);
    return 0;
}
