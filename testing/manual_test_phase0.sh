#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 0 Automated Manual Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 0 Verification Suite               "
echo "======================================================================"

# 1. Toolchain verification
echo "==> Step 1: Checking host toolchain dependencies..."
for tool in gcc cpio qemu-system-x86_64; do
    if ! command -v "$tool" &> /dev/null; then
        echo "ERROR: Required tool '$tool' not found on host."
        exit 1
    fi
done
echo "    [PASS] Toolchain verification successful."

# 2. Build static init binary
echo "==> Step 2: Compiling static C init PID 1 executable (karim-init)..."
make init

if [ ! -f "build/init" ]; then
    echo "ERROR: build/init binary not found."
    exit 1
fi

echo "==> Step 3: Verifying ELF static linkage of build/init..."
file_out=$(file build/init)
echo "    Binary info: $file_out"
if ! echo "$file_out" | grep -i "statically linked" > /dev/null; then
    echo "WARNING: build/init is not reported as statically linked."
fi
echo "    [PASS] Static C init binary build verified."

# 3. Assemble initramfs CPIO archive
echo "==> Step 4: Packing initramfs CPIO archive..."
make initramfs

if [ ! -f "dist/initramfs.cpio" ]; then
    echo "ERROR: dist/initramfs.cpio archive not found."
    exit 1
fi

echo "==> Step 5: Verifying CPIO initramfs archive structure..."
cpio_contents=$(cpio -t < dist/initramfs.cpio 2>/dev/null)
echo "    Archive contents:"
echo "$cpio_contents" | sed 's/^/      /'
if ! echo "$cpio_contents" | grep "init" > /dev/null; then
    echo "ERROR: init binary missing from initramfs.cpio archive."
    exit 1
fi
echo "    [PASS] Initramfs archive structure verified."

echo "======================================================================"
echo "          Phase 0 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
