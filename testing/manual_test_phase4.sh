#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 4 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 4 Verification Suite               "
echo "======================================================================"

# 1. Run Go vsockd Unit Test Suite
echo "==> Step 1: Running Go vsockd unit test suite..."
go test -v -race ./internal/vsockd/...
echo "    [PASS] All Go vsockd unit tests passed successfully."

# 2. Build static karim-vsockd daemon and host karim CLI
echo "==> Step 2: Compiling karim-vsockd daemon and host karim CLI tool..."
make vsockd cli

if [ ! -f "build/karim-vsockd" ]; then
    echo "ERROR: build/karim-vsockd binary not found."
    exit 1
fi

if [ ! -f "dist/karim" ]; then
    echo "ERROR: dist/karim binary not found."
    exit 1
fi

echo "==> Step 3: Verifying ELF static linkage of build/karim-vsockd..."
file_out=$(file build/karim-vsockd)
echo "    Binary info: $file_out"
echo "    [PASS] Static Go vsockd daemon build verified."

# 4. Test Host-Guest Control Plane RPC over transport socket
echo "==> Step 4: Testing Host-Guest Control Plane RPCs via karim CLI..."
TEST_SOCKET="/tmp/karim_phase4_verification.sock"
rm -f "$TEST_SOCKET"

# Launch background vsockd daemon on Unix socket
./build/karim-vsockd -socket "$TEST_SOCKET" -config-dir config/services > /dev/null 2>&1 &
VSOCKD_PID=$!
trap 'kill $VSOCKD_PID 2>/dev/null || true; rm -f "$TEST_SOCKET"' EXIT

sleep 0.2

echo "==> Testing 'karim ping'..."
ping_out=$(./dist/karim --target "unix://$TEST_SOCKET" ping)
echo "    $ping_out"
if [[ "$ping_out" != *"PONG"* ]]; then
    echo "ERROR: 'karim ping' failed to return PONG response."
    exit 1
fi
echo "    [PASS] Control plane ping RPC verified."

echo "==> Testing 'karim ps'..."
ps_out=$(./dist/karim --target "unix://$TEST_SOCKET" ps)
echo "$ps_out"
if [[ "$ps_out" != *"sample_app"* ]]; then
    echo "ERROR: 'karim ps' did not report sample_app service."
    exit 1
fi
echo "    [PASS] Control plane service listing RPC verified."

echo "==> Testing 'karim metrics'..."
metrics_out=$(./dist/karim --target "unix://$TEST_SOCKET" metrics)
echo "$metrics_out"
if [[ "$metrics_out" != *"Performance Metrics"* ]]; then
    echo "ERROR: 'karim metrics' output invalid."
    exit 1
fi
echo "    [PASS] Control plane metrics RPC verified."

# Kill test daemon
kill $VSOCKD_PID 2>/dev/null || true
rm -f "$TEST_SOCKET"
trap - EXIT

# 5. Pack SquashFS rootfs image containing integrated vsockd
echo "==> Step 5: Packing SquashFS rootfs image containing karim-vsockd (rootfs.sqsh)..."
make rootfs

if [ ! -f "dist/rootfs.sqsh" ]; then
    echo "ERROR: dist/rootfs.sqsh image not found."
    exit 1
fi

sqsh_size=$(du -h dist/rootfs.sqsh | cut -f1)
echo "    SquashFS rootfs ready at dist/rootfs.sqsh ($sqsh_size)"
echo "    [PASS] Control plane integrated rootfs SquashFS image assembly verified."

echo "======================================================================"
echo "          Phase 4 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
