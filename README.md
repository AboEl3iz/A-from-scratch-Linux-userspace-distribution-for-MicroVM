# Karim MicroVM OS

**Karim MicroVM OS** is a zero-dependency, minimal Linux distribution built from scratch for high-performance MicroVM workloads. It provides a static C PID 1 boot loader (`karim-init`), a compiled Go supervisor (`karim-svcd`) for process hierarchy management with cgroups v2 isolation, netlink networking (`karim-netd`), OverlayFS storage (`karim-stored`), Seccomp BPF & capability isolation (`karim-secd`), host-guest VSOCK control plane (`karim-vsockd` & `karim` CLI), eBPF CO-RE observability, and reproducible image building.

---

## Architecture Overview

```
       ┌──────────────────────────────────────────────────────────┐
       │ Host Management CLI (karim)                              │
       │ - Communicates via AF_VSOCK (vsock://3:1024)             │
       └────────────────────────────┬─────────────────────────────┘
                                    │ virtio-vsock stream
       ┌────────────────────────────▼─────────────────────────────┐
       │ Linux Kernel (bzImage) + Initramfs                       │
       └────────────────────────────┬─────────────────────────────┘
                                    │
       ┌────────────────────────────▼─────────────────────────────┐
       │ PID 1: Static C Init (karim-init)                        │
       │ - Mounts /proc, /sys, /dev, /sys/fs/cgroup               │
       │ - Reaps zombie processes asynchronously (SIGCHLD)        │
       └────────────────────────────┬─────────────────────────────┘
                                    │
       ┌────────────────────────────▼─────────────────────────────┐
       │ Secondary Supervisor: Go Supervisor (karim-svcd)         │
       │ - Hosts VSOCK RPC control plane (karim-vsockd)           │
       │ - Initializes virtio-net networking (karim-netd)         │
       │ - Mounts SquashFS + tmpfs OverlayFS (karim-stored)       │
       │ - Enforces Seccomp BPF & Capabilities (karim-secd)      │
       │ - Parses /etc/karim/services/*.toml                       │
       │ - Resolves service DAG startup graph                     │
       │ - Manages cgroup v2 leaves (/sys/fs/cgroup/karim/<svc>)  │
       │ - Captures process stdout/stderr logs                    │
       └──────────────────────────────────────────────────────────┘
```

---

## Phase Status & Roadmap

| Phase | Description | Status | Deliverables / Verification |
|---|---|---|---|
| **Phase 0** | Minimal Static C Init (`karim-init`) | **COMPLETED** | Static `init`, initramfs, `make test-phase0` |
| **Phase 1** | Go Supervisor (`karim-svcd`) & Process Hierarchy | **COMPLETED** | `karim-svcd`, cgroups v2, DAG resolver, `make test-phase1` |
| **Phase 2** | Networking (`karim-netd`) & Storage Overlay (`karim-stored`) | **COMPLETED** | Netlink netns, rtnetlink bringup, OverlayFS, `make test-phase2` |
| **Phase 3** | Hardened Isolation (`karim-secd`) | **COMPLETED** | `karim-secd`, Seccomp BPF filters, Linux capabilities, `make test-phase3` |
| **Phase 4** | Host-Guest Control Plane (`karim-vsockd` & `karim` CLI) | **COMPLETED** | `AF_VSOCK` RPC server, host CLI `dist/karim`, `make test-phase4` |
| **Phase 5** | eBPF Observability Engine (`karim-obsd`) | **COMPLETED** | eBPF CO-RE probes (`bpf2go`), Prometheus exporter, `make test-phase5` |
| **Phase 6** | Hermetic Reproducible Image Builder | **COMPLETED** | Hermetic CPIO/SquashFS builder, SHA-256 manifest, `make test-phase6` |
| **Phase 7** | Build-Time Layer Engine (`karim-pkgd`) | **COMPLETED** | OCI tarball parser, whiteout engine, `make test-phase7` |
| **Phase 8** | QMP Snapshot & Restore Orchestration | Planned | QEMU QMP memory serialization |
| **Phase 9** | Entropy, RTC Sync & Debug Hardening | Planned | `virtio-rng`, RTC sync, debug shell |

---

## Quick Start & Building

### Prerequisites

Ensure essential build tools are installed on your Linux host:
```bash
sudo apt-get update && sudo apt-get install -y \
    gcc musl-tools clang qemu-system-x86 squashfs-tools cpio
```

### Build Commands

```bash
# Check toolchain availability
make check-tools

# Build static C PID 1 init binary (Phase 0)
make init

# Build static Go supervisor binary (Phase 1)
make svcd

# Build static Go pre-exec isolation launcher binary (Phase 3)
make secd

# Build guest control plane daemon binary (Phase 4)
make vsockd

# Build host-side karim CLI management tool (Phase 4)
make cli

# Generate Go bindings from eBPF C probes via bpf2go (Phase 5)
make ebpf

# Build static Go eBPF observability daemon binary (Phase 5)
make obsd

# Pack initramfs CPIO archive
make initramfs

# Assemble SquashFS root filesystem
make rootfs

# Build hermetic reproducible images via karim CLI (Phase 6)
make build-hermetic

# Perform double-run byte-for-byte reproducibility audit (Phase 6)
make verify-reproducible

# Run host-side karim CLI tool with custom arguments
make run-cli ARGS="help"

# Build all Karim MicroVM OS artifacts
make all
```

---

## Verification & Testing

Karim OS includes dedicated automated test suites for host unit testing and MicroVM integration testing.

### Running Phase 0 Verification

Verifies static compilation of `karim-init`, ELF linkage, and CPIO initramfs archive structure:
```bash
make test-phase0
```

### Running Phase 1 Verification

Executes Go unit tests for TOML configuration parsing, cgroup v2 subtree creation, and DAG topological sorting:
```bash
make test-phase1
```

### Running Phase 2 Verification

Executes Go unit tests for netlink interface configuration and OverlayFS mount options:
```bash
make test-phase2
```

### Running Phase 3 Verification

Executes Go unit tests for Seccomp BPF bytecode compilation, capability dropping, and forbidden syscall trap verification:
```bash
make test-phase3
```

### Running Phase 4 Verification

Executes end-to-end tests for `AF_VSOCK` transport abstraction, framed JSON RPC dispatch, and host CLI (`karim ping`, `karim ps`, `karim metrics`):
```bash
make test-phase4
```

### Running Phase 5 Verification

Executes end-to-end tests for eBPF kernel latency probes, `karim-obsd` daemon RPCs, Prometheus `/metrics` HTTP exporter, and host CLI (`karim obsd`, `karim trace`):
```bash
make test-phase5
```

### Running Phase 6 Verification

Executes Go builder unit tests for deterministic CPIO header generation and manifest hashing, builds hermetic `initramfs.cpio` and `rootfs.sqsh` image artifacts along with `manifest.json`, and asserts 100% byte-for-byte SHA-256 identity across double-build clean runs:
```bash
make test-phase6
```

### Running Phase 7 Verification

Executes Go pkgd unit tests for OCI tarball whiteout resolution (`.wh.<file>` & `.wh..wh..opq`) and layer config loading, inspects synthetic multi-layer OCI archives, and compiles integrated SquashFS rootfs images:
```bash
make test-phase7
```

### Running MicroVM inside QEMU

Boot the microVM in debug mode (kernel logs visible):
```bash
make run-debug
```

---

## Project Structure

```
karim-microvm-os/
├── Makefile                     # Central build & test automation script
├── README.md                    # Project documentation & phase tracking
├── KARIM_DEVELOPMENT_PLAN.md    # Master architecture specification
├── cmd/
│   ├── karim/                   # Host-side management CLI tool (main.go)
│   ├── karim-obsd/              # eBPF kernel observability daemon (main.go)
│   ├── karim-secd/              # Pre-exec security launcher binary (main.go)
│   ├── karim-svcd/              # Go PID 1 successor supervisor (main.go)
│   └── karim-vsockd/            # Guest control plane daemon entrypoint (main.go)
├── ebpf/                        # eBPF C programs & bpf2go generator
│   ├── biolatency.bpf.c         # Block I/O latency histogram probe
│   ├── ebpf.go                  # //go:generate bpf2go directive
│   ├── execsnoop.bpf.c          # Process execution tracing probe
│   └── runqlat.bpf.c            # CPU scheduler runqueue latency probe
├── init/
│   ├── init.c                   # Static C init binary (PID 1)
│   └── sample_app.c             # Sample static workload application
├── internal/
│   ├── builder/                 # Hermetic image compiler, CPIO & SquashFS packers, manifest generator
│   ├── netd/                    # Netlink network configuration subsystem
│   ├── obsd/                    # eBPF ring buffer reader & Prometheus exporter
│   ├── pkgd/                    # OCI container image parser, whiteout engine, & layer delta merger
│   ├── secd/                    # Seccomp BPF filter compiler & capabilities engine
│   ├── stored/                  # OverlayFS storage management subsystem
│   ├── svcd/                    # Supervisor modules (config, graph, cgroup, supervisor)
│   └── vsockd/                  # Host-Guest VSOCK RPC server & transport abstraction
├── config/
│   └── services/                # TOML service configuration files
│       ├── karim-obsd.toml
│       └── sample_app.toml
└── testing/
    ├── manual_test_phase0.sh    # Automated Phase 0 test harness
    ├── manual_test_phase1.sh    # Automated Phase 1 test harness
    ├── manual_test_phase2.sh    # Automated Phase 2 test harness
    ├── manual_test_phase3.sh    # Automated Phase 3 test harness
    ├── manual_test_phase4.sh    # Automated Phase 4 test harness
    ├── manual_test_phase5.sh    # Automated Phase 5 test harness
    ├── manual_test_phase6.sh    # Automated Phase 6 test harness
    └── manual_test_phase7.sh    # Automated Phase 7 test harness
```
