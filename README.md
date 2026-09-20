# Karim MicroVM OS

[![Linux Kernel](https://img.shields.io/badge/Linux_Kernel-v6.6-blue?style=for-the-badge&logo=linux&logoColor=white)](https://kernel.org)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen?style=for-the-badge&logo=githubactions&logoColor=white)](#verification--testing)
[![Reproducibility](https://img.shields.io/badge/Reproducible_Builds-100%25_SHA--256-success?style=for-the-badge&logo=reproduciblebuilds&logoColor=white)](#running-phase-6-verification)
[![Architecture](https://img.shields.io/badge/Architecture-x86__64_MicroVM-orange?style=for-the-badge&logo=qemu&logoColor=white)](#architecture-overview)
[![Security](https://img.shields.io/badge/Security-Seccomp_BPF_%2B_Caps-red?style=for-the-badge&logo=shield&logoColor=white)](#running-phase-3-verification)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](#)

**Karim MicroVM OS** is a zero-dependency, minimal Linux distribution built from scratch for high-performance MicroVM workloads. It provides a static C PID 1 boot loader (`karim-init`), a compiled Go supervisor (`karim-svcd`) for process hierarchy management with cgroups v2 isolation, netlink networking (`karim-netd`), OverlayFS storage (`karim-stored`), Seccomp BPF & capability isolation (`karim-secd`), host-guest VSOCK control plane (`karim-vsockd` & `karim` CLI), eBPF CO-RE observability, reproducible image building, OCI layer engine, QMP state snapshot/restore orchestration, and hardware entropy / RTC clock sync / debug hardening (`internal/system`).

---

## Architecture Overview

```
       ┌──────────────────────────────────────────────────────────┐
       │ Host Management CLI (karim)                              │
       │ - Communicates via AF_VSOCK (vsock://3:1024)             │
       │ - Orchestrates QMP Snapshots (unix:///tmp/qmp.sock)     │
       │ - Queries System Entropy, RTC Sync & Debug Mode          │
       └────────────────────────────┬─────────────────────────────┘
                                    │ virtio-vsock stream / QMP
       ┌────────────────────────────▼─────────────────────────────┐
       │ Linux Kernel (bzImage) + Initramfs                       │
       └────────────────────────────┬─────────────────────────────┘
                                    │
       ┌────────────────────────────▼─────────────────────────────┐
       │ PID 1: Static C Init (karim-init)                        │
       │ - Mounts /proc, /sys, /dev, /sys/fs/cgroup               │
       │ - Seeds entropy pool from virtio-rng (/dev/hwrng)        │
       │ - Synchronizes hardware Real-Time Clock (/dev/rtc0)       │
       │ - Parses /proc/cmdline for emergency debug shell         │
       │ - Reaps zombie processes asynchronously (SIGCHLD)        │
       └────────────────────────────┬─────────────────────────────┘
                                    │
       ┌────────────────────────────▼─────────────────────────────┐
       │ Secondary Supervisor: Go Supervisor (karim-svcd)         │
       │ - Hosts VSOCK RPC control plane (karim-vsockd)           │
       │ - Initializes virtio-net networking (karim-netd)         │
       │ - Mounts SquashFS + tmpfs OverlayFS (karim-stored)       │
       │ - Enforces Seccomp BPF & Capabilities (karim-secd)      │
       │ - Exposes system entropy & RTC health over VSOCK         │
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
| **Phase 7** | Build-Time Layer Engine (`internal/pkgd`) | **COMPLETED** | OCI tarball parser, whiteout engine, `karim import`, `make test-phase7` |
| **Phase 8** | QMP Snapshot & Restore Orchestration (`internal/qmp` & `internal/snapshot`) | **COMPLETED** | QEMU QMP memory state save/restore, guest VFS quiesce via `syscall.Sync`, `karim snapshot`, `make test-phase8` |
| **Phase 9** | Entropy, RTC Sync & Debug Hardening (`internal/system`) | **COMPLETED** | `virtio-rng` seed, RTC `/dev/rtc0` sync, `/proc/cmdline` debug shell, `karim system`, `make test-phase9` |


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

### Running Phase 8 Verification

Executes Go unit tests for QMP hypervisor IPC connection handling, state save/restore commands, guest VFS buffer quiescing via `syscall.Sync()`, and `karim snapshot` CLI orchestration:
```bash
make test-phase8
```

### Running Phase 9 Verification

Executes Go unit tests for internal system entropy parsing, kernel cmdline debug flag detection, static `init.c` binary string auditing (virtio-rng, RTC sync, emergency debug shell), mock VSOCK RPC server initialization, and `karim system` host CLI reporting:
```bash
make test-phase9
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
│   └── init.c                   # Static C init binary (PID 1)
├── internal/
│   ├── builder/                 # Hermetic image compiler, CPIO & SquashFS packers, manifest generator
│   ├── netd/                    # Netlink network configuration subsystem
│   ├── obsd/                    # eBPF ring buffer reader & Prometheus exporter
│   ├── pkgd/                    # OCI container image parser, whiteout engine, & layer delta merger
│   ├── qmp/                     # QEMU QMP socket client & command encoder
│   ├── secd/                    # Seccomp BPF filter compiler & capabilities engine
│   ├── snapshot/                # MicroVM state save/restore/list orchestrator
│   ├── stored/                  # OverlayFS storage management subsystem
│   ├── svcd/                    # Supervisor modules (config, graph, cgroup, supervisor)
│   ├── system/                  # System entropy, RTC availability & cmdline debug flag engine
│   └── vsockd/                  # Host-Guest VSOCK RPC server & transport abstraction
├── config/
│   └── services/                # TOML service configuration files
│       └── karim-obsd.toml
└── testing/
    ├── manual_test_phase0.sh    # Automated Phase 0 test harness
    ├── manual_test_phase1.sh    # Automated Phase 1 test harness
    ├── manual_test_phase2.sh    # Automated Phase 2 test harness
    ├── manual_test_phase3.sh    # Automated Phase 3 test harness
    ├── manual_test_phase4.sh    # Automated Phase 4 test harness
    ├── manual_test_phase5.sh    # Automated Phase 5 test harness
    ├── manual_test_phase6.sh    # Automated Phase 6 test harness
    ├── manual_test_phase7.sh    # Automated Phase 7 test harness
    ├── manual_test_phase8.sh    # Automated Phase 8 test harness
    └── manual_test_phase9.sh    # Automated Phase 9 test harness
```

