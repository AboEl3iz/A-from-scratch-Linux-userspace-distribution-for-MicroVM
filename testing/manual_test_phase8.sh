#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 8 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 8 Verification Suite               "
echo "======================================================================"

# 1. Run internal qmp and snapshot package unit tests
echo "==> Step 1: Running internal/qmp & internal/snapshot Go unit test suites..."
go test -v ./internal/qmp/... ./internal/snapshot/...
echo "    [PASS] All Go QMP & Snapshot unit tests passed successfully."

# 2. Build host CLI and guest daemons
echo "==> Step 2: Compiling host karim CLI and microVM daemons..."
make init svcd secd vsockd obsd cli

if [ ! -f "dist/karim" ]; then
    echo "ERROR: dist/karim host CLI binary missing."
    exit 1
fi

echo "    [PASS] Host CLI build verified."

# 3. Spawn background test harness with mock QMP and VSOCK Unix domain sockets
echo "==> Step 3: Starting mock QMP & VSOCK socket test servers..."
TEST_DIR="/tmp/karim_phase8_test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

QMP_SOCK="$TEST_DIR/qmp.sock"
VSOCK_SOCK="$TEST_DIR/vsock.sock"

# Start background Go mock server test runner
go run -v ./testing/mock_qmp_server.go -qmp "$QMP_SOCK" -vsock "$VSOCK_SOCK" &
MOCK_PID=$!

# Ensure cleanup of mock server on exit
trap 'kill -9 $MOCK_PID 2>/dev/null || true; rm -rf "$TEST_DIR"' EXIT

# Wait for sockets to open
sleep 1

if [ ! -S "$QMP_SOCK" ]; then
    echo "ERROR: Mock QMP socket failed to initialize at $QMP_SOCK"
    exit 1
fi

echo "    [PASS] Mock QMP & VSOCK socket servers running."

# 4. Test 'karim snapshot status'
echo "==> Step 4: Testing 'karim snapshot status'..."
status_out=$(./dist/karim snapshot status --qmp-socket "$QMP_SOCK")
echo "$status_out"

if [[ "$status_out" != *"running"* ]]; then
    echo "ERROR: 'karim snapshot status' failed."
    exit 1
fi
echo "    [PASS] 'karim snapshot status' verified."

# 5. Test 'karim snapshot save' (with VSOCK guest quiesce)
echo "==> Step 5: Testing 'karim snapshot save' with guest VFS quiesce..."
save_out=$(./dist/karim --target "unix://$VSOCK_SOCK" snapshot save "phase8-snap-v1" --qmp-socket "$QMP_SOCK")
echo "$save_out"

if [[ "$save_out" != *"Saved Successfully"* || "$save_out" != *"phase8-snap-v1"* ]]; then
    echo "ERROR: 'karim snapshot save' failed."
    exit 1
fi
echo "    [PASS] 'karim snapshot save' verified."

# 6. Test 'karim snapshot list'
echo "==> Step 6: Testing 'karim snapshot list'..."
list_out=$(./dist/karim snapshot list --qmp-socket "$QMP_SOCK")
echo "$list_out"

if [[ "$list_out" != *"phase8-snap-v1"* ]]; then
    echo "ERROR: 'karim snapshot list' failed."
    exit 1
fi
echo "    [PASS] 'karim snapshot list' verified."

# 7. Test 'karim snapshot restore'
echo "==> Step 7: Testing 'karim snapshot restore'..."
restore_out=$(./dist/karim --target "unix://$VSOCK_SOCK" snapshot restore "phase8-snap-v1" --qmp-socket "$QMP_SOCK")
echo "$restore_out"

if [[ "$restore_out" != *"Restored Successfully"* ]]; then
    echo "ERROR: 'karim snapshot restore' failed."
    exit 1
fi
echo "    [PASS] 'karim snapshot restore' verified."

# 8. Test 'karim snapshot delete'
echo "==> Step 8: Testing 'karim snapshot delete'..."
del_out=$(./dist/karim snapshot delete "phase8-snap-v1" --qmp-socket "$QMP_SOCK")
echo "$del_out"

if [[ "$del_out" != *"deleted successfully"* ]]; then
    echo "ERROR: 'karim snapshot delete' failed."
    exit 1
fi
echo "    [PASS] 'karim snapshot delete' verified."

echo "======================================================================"
echo "          Phase 8 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
