// +build ignore

#include "vmlinux.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

char LICENSE[] SEC("license") = "GPL";

struct req_key {
    dev_t dev;
    unsigned long long sector;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, struct req_key);
    __type(value, __u64);
} start_req SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct hist);
} io_lat SEC(".maps");

static __always_inline __u64 log2u(__u64 v)
{
    __u64 r = 0;
    while (v >>= 1) {
        r++;
    }
    return r;
}

SEC("tp/block/block_rq_issue")
int handle_block_rq_issue(struct trace_event_raw_block_rq_issue *ctx)
{
    struct req_key key = {
        .dev = ctx->dev,
        .sector = ctx->sector,
    };
    __u64 ts = bpf_ktime_get_ns();

    bpf_map_update_elem(&start_req, &key, &ts, BPF_ANY);
    return 0;
}

SEC("tp/block/block_rq_complete")
int handle_block_rq_complete(struct trace_event_raw_block_rq_complete *ctx)
{
    struct req_key key = {
        .dev = ctx->dev,
        .sector = ctx->sector,
    };
    __u64 *tsp, delta, now, slot;
    __u32 zero = 0;
    struct hist *histp;

    tsp = bpf_map_lookup_elem(&start_req, &key);
    if (!tsp)
        return 0;

    now = bpf_ktime_get_ns();
    if (now > *tsp) {
        delta = (now - *tsp) / 1000; // microseconds
        slot = log2u(delta);
        if (slot >= MAX_SLOTS)
            slot = MAX_SLOTS - 1;

        histp = bpf_map_lookup_elem(&io_lat, &zero);
        if (histp) {
            __sync_fetch_and_add(&histp->slots[slot], 1);
        }
    }

    bpf_map_delete_elem(&start_req, &key);
    return 0;
}
