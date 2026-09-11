// +build ignore

#include "vmlinux.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);
    __type(value, __u64);
} start SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct hist);
} runq_lat SEC(".maps");

static __always_inline __u64 log2u(__u64 v)
{
    __u64 r = 0;
    while (v >>= 1) {
        r++;
    }
    return r;
}

SEC("tp/sched/sched_wakeup")
int handle_sched_wakeup(struct trace_event_raw_sched_wakeup *ctx)
{
    __u32 pid = ctx->pid;
    __u64 ts = bpf_ktime_get_ns();

    if (pid == 0)
        return 0;

    bpf_map_update_elem(&start, &pid, &ts, BPF_ANY);
    return 0;
}

SEC("tp/sched/sched_switch")
int handle_sched_switch(struct trace_event_raw_sched_switch *ctx)
{
    __u32 next_pid = ctx->next_pid;
    __u64 *tsp, delta, now, slot;
    __u32 zero = 0;
    struct hist *histp;

    if (next_pid == 0)
        return 0;

    tsp = bpf_map_lookup_elem(&start, &next_pid);
    if (!tsp)
        return 0;

    now = bpf_ktime_get_ns();
    if (now > *tsp) {
        delta = (now - *tsp) / 1000; // microseconds
        slot = log2u(delta);
        if (slot >= MAX_SLOTS)
            slot = MAX_SLOTS - 1;

        histp = bpf_map_lookup_elem(&runq_lat, &zero);
        if (histp) {
            __sync_fetch_and_add(&histp->slots[slot], 1);
        }
    }

    bpf_map_delete_elem(&start, &next_pid);
    return 0;
}
