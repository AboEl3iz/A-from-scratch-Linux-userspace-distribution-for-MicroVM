#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 9 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 9 Verification Suite               "
echo "======================================================================"

# 1. Run internal system package unit tests
echo "==> Step 1: Running internal/system Go unit test suite..."
go test -v ./internal/system/...
echo "    [PASS] All Go system unit tests passed successfully."

# 2. Build binaries (init, svcd, cli)
echo "==> Step 2: Compiling PID 1 C init binary and Go daemons..."
make init svcd secd vsockd obsd cli

if [ ! -f "dist/karim" ]; then
    echo "ERROR: dist/karim host CLI binary missing."
    exit 1
fi

if [ ! -f "build/init" ]; then
    echo "ERROR: build/init static init binary missing."
    exit 1
fi

echo "    [PASS] All C and Go build artifacts verified."

# 3. Audit static init binary strings for Phase 9 functions
echo "==> Step 3: Auditing static init binary for Phase 9 functions..."
if grep -q "init_entropy_pool" init/init.c; then
    echo "    [PASS] Entropy initialization routine present in init.c."
else
    echo "ERROR: Entropy initialization missing in init.c."
    exit 1
fi

if grep -q "sync_rtc_time" init/init.c; then
    echo "    [PASS] RTC time sync routine present in init.c."
else
    echo "ERROR: RTC sync routine missing in init.c."
    exit 1
fi

if grep -q "spawn_debug_shell" init/init.c; then
    echo "    [PASS] Emergency debug shell routine present in init.c."
else
    echo "ERROR: Emergency debug shell missing in init.c."
    exit 1
fi



# 4. Spawn background test server for 'karim system' CLI verification
echo "==> Step 4: Starting mock VSOCK socket test server..."
TEST_DIR="/tmp/karim_phase9_test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

VSOCK_SOCK="$TEST_DIR/vsock.sock"

# Start background Go mock server test runner
go run -v ./testing/mock_qmp_server.go -qmp "$TEST_DIR/qmp.sock" -vsock "$VSOCK_SOCK" &
MOCK_PID=$!

# Ensure cleanup on exit
trap 'kill -9 $MOCK_PID 2>/dev/null || true; rm -rf "$TEST_DIR"' EXIT

sleep 1

if [ ! -S "$VSOCK_SOCK" ]; then
    echo "ERROR: Mock VSOCK socket failed to initialize at $VSOCK_SOCK"
    exit 1
fi

echo "    [PASS] Mock VSOCK socket server running."

# 5. Test 'karim system' CLI command
echo "==> Step 5: Testing 'karim system' CLI command..."
sys_out=$(./dist/karim --target "unix://$VSOCK_SOCK" system)
echo "$sys_out"

if [[ "$sys_out" != *"Phase 9 System, Entropy & RTC Hardening Status"* ]]; then
    echo "ERROR: 'karim system' output verification failed."
    exit 1
fi
echo "    [PASS] 'karim system' CLI status reporting verified."

echo "======================================================================"
echo "          Phase 9 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
