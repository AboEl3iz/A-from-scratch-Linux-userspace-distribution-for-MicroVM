package ebpf

//go:generate bpf2go -target amd64 Execsnoop execsnoop.bpf.c -- -I. -I/usr/include -I/usr/include/x86_64-linux-gnu
//go:generate bpf2go -target amd64 Runqlat runqlat.bpf.c -- -I. -I/usr/include -I/usr/include/x86_64-linux-gnu
//go:generate bpf2go -target amd64 Biolatency biolatency.bpf.c -- -I. -I/usr/include -I/usr/include/x86_64-linux-gnu


