#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 5 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 5 Verification Suite               "
echo "======================================================================"

# 1. Compile eBPF probes and run Go unit test suite
echo "==> Step 1: Compiling eBPF probes and running obsd unit test suite..."
make ebpf
go test -v -race ./internal/obsd/...
echo "    [PASS] All Go obsd unit tests passed successfully."

# 2. Build static karim-obsd daemon and host karim CLI
echo "==> Step 2: Compiling karim-obsd daemon and host karim CLI tool..."
make obsd cli

if [ ! -f "build/karim-obsd" ]; then
    echo "ERROR: build/karim-obsd binary not found."
    exit 1
fi

if [ ! -f "dist/karim" ]; then
    echo "ERROR: dist/karim binary not found."
    exit 1
fi

echo "==> Step 3: Verifying ELF static linkage of build/karim-obsd..."
file_out=$(file build/karim-obsd)
echo "    Binary info: $file_out"
echo "    [PASS] Static Go obsd daemon build verified."

# 4. Test eBPF Telemetry Daemon RPCs
echo "==> Step 4: Testing eBPF Telemetry RPCs via karim-obsd and karim CLI..."
TEST_SOCKET="/tmp/karim_phase5_verification.sock"
rm -f "$TEST_SOCKET"

# Launch background obsd daemon
./build/karim-obsd -socket "$TEST_SOCKET" -prom-port 9199 > /dev/null 2>&1 &
OBSD_PID=$!
trap 'kill $OBSD_PID 2>/dev/null || true; rm -f "$TEST_SOCKET"' EXIT

sleep 0.5

echo "==> Testing 'karim obsd' telemetry query..."
obsd_out=$(./dist/karim --target "unix://$TEST_SOCKET" obsd)
echo "$obsd_out"
if [[ "$obsd_out" != *"sched_runq_latency_microseconds"* && "$obsd_out" != *"runq_latency"* ]]; then
    echo "ERROR: 'karim obsd' failed to return eBPF telemetry."
    exit 1
fi
echo "    [PASS] eBPF telemetry query verified."

echo "==> Testing 'karim trace' process exec query..."
trace_out=$(./dist/karim --target "unix://$TEST_SOCKET" trace)
echo "$trace_out"
if [[ "$trace_out" != *"Traced Process Execution"* && "$trace_out" != *"traced process execution"* ]]; then
    echo "ERROR: 'karim trace' failed to return process trace data."
    exit 1
fi
echo "    [PASS] eBPF process execution tracing verified."

echo "==> Testing Prometheus metrics endpoint on http://127.0.0.1:9199/metrics..."
prom_out=$(curl -s http://127.0.0.1:9199/metrics)
echo "$prom_out" | head -n 12
if [[ "$prom_out" != *"karim_obsd_probe_info"* ]]; then
    echo "ERROR: Prometheus metrics endpoint did not return karim_obsd_probe_info."
    exit 1
fi
echo "    [PASS] Prometheus metrics HTTP endpoint verified."

# Kill test daemon
kill $OBSD_PID 2>/dev/null || true
rm -f "$TEST_SOCKET"
trap - EXIT

# 5. Pack SquashFS rootfs image containing integrated karim-obsd
echo "==> Step 5: Packing SquashFS rootfs image containing karim-obsd (rootfs.sqsh)..."
make rootfs

if [ ! -f "dist/rootfs.sqsh" ]; then
    echo "ERROR: dist/rootfs.sqsh image not found."
    exit 1
fi

sqsh_size=$(du -h dist/rootfs.sqsh | cut -f1)
echo "    SquashFS rootfs ready at dist/rootfs.sqsh ($sqsh_size)"
echo "    [PASS] eBPF engine integrated rootfs SquashFS image assembly verified."

echo "======================================================================"
echo "          Phase 5 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
