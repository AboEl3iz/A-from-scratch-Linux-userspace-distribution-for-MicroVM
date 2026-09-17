#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 6 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 6 Verification Suite               "
echo "======================================================================"

# 1. Run builder package unit tests
echo "==> Step 1: Running Hermetic Builder Go unit test suite..."
go test -v ./internal/builder/...
echo "    [PASS] All Go builder unit tests passed successfully."

# 2. Build binaries required for hermetic image packaging
echo "==> Step 2: Compiling init, svcd, secd, vsockd, obsd daemons and karim CLI..."
make init svcd secd vsockd obsd cli

if [ ! -f "build/init" ] || [ ! -f "build/karim-svcd" ] || [ ! -f "dist/karim" ]; then
    echo "ERROR: Required build binaries missing."
    exit 1
fi

echo "    [PASS] Required build binaries verified."

# 3. Test Hermetic Image Generation
echo "==> Step 3: Generating Hermetic Image artifacts (initramfs.cpio, rootfs.sqsh, manifest.json)..."
./dist/karim build --out-dir="dist" --init-bin="build/init" --svcd-bin="build/karim-svcd"

if [ ! -f "dist/initramfs.cpio" ]; then
    echo "ERROR: dist/initramfs.cpio not found."
    exit 1
fi

if [ ! -f "dist/rootfs.sqsh" ]; then
    echo "ERROR: dist/rootfs.sqsh not found."
    exit 1
fi

if [ ! -f "dist/manifest.json" ]; then
    echo "ERROR: dist/manifest.json not found."
    exit 1
fi

echo "    [PASS] Hermetic Image build completed successfully."
echo "==> Generated Manifest Content:"
cat dist/manifest.json

# 4. Perform Double-Build Reproducibility Verification
echo "==> Step 4: Performing Double-Build Byte-for-Byte Reproducibility Audit..."
./dist/karim build --out-dir="dist" --init-bin="build/init" --svcd-bin="build/karim-svcd" --verify-reproducible

echo "    [PASS] 100% Byte-for-Byte Reproducibility Verified!"

echo "======================================================================"
echo "          Phase 6 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
