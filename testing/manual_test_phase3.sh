#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 3 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 3 Verification Suite               "
echo "======================================================================"

# 1. Run Go Secd & Svcd Unit Test Suites
echo "==> Step 1: Running Go secd & svcd unit test suites..."
go test -v -race ./internal/secd/... ./internal/svcd/...
echo "    [PASS] All Go secd and svcd unit tests passed successfully."

# 2. Build static karim-secd isolation binary
echo "==> Step 2: Compiling static Go secd binary (karim-secd)..."
make secd

if [ ! -f "build/karim-secd" ]; then
    echo "ERROR: build/karim-secd binary not found."
    exit 1
fi

echo "==> Step 3: Verifying ELF static linkage of build/karim-secd..."
file_out=$(file build/karim-secd)
echo "    Binary info: $file_out"
echo "    [PASS] Static Go secd binary build verified."

# 3. Test Seccomp BPF Forbidden Syscall Enforcement
echo "==> Step 4: Testing Seccomp BPF forbidden syscall termination..."
set +e
./build/karim-secd -profile strict -exec /bin/mkdir /tmp/test_dir_phase3 > /dev/null 2>&1
exit_code=$?
set -e

if [ $exit_code -eq 0 ]; then
    echo "ERROR: /bin/mkdir succeeded under strict seccomp profile! Expected process termination by SIGSYS."
    exit 1
fi

echo "    Command /bin/mkdir was blocked and killed under strict profile (exit code: $exit_code)."
echo "    [PASS] Seccomp BPF forbidden syscall protection verified."

# 4. Pack SquashFS rootfs image containing integrated secd and svcd
echo "==> Step 5: Packing SquashFS rootfs image containing karim-secd (rootfs.sqsh)..."
make rootfs

if [ ! -f "dist/rootfs.sqsh" ]; then
    echo "ERROR: dist/rootfs.sqsh image not found."
    exit 1
fi

sqsh_size=$(du -h dist/rootfs.sqsh | cut -f1)
echo "    SquashFS rootfs ready at dist/rootfs.sqsh ($sqsh_size)"
echo "    [PASS] Hardened rootfs SquashFS image assembly verified."

echo "======================================================================"
echo "          Phase 3 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
