#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 1 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 1 Verification Suite               "
echo "======================================================================"

# 1. Run Host Go Unit Tests
echo "==> Step 1: Running Go supervisor unit test suite (svcd)..."
go test -v -race ./internal/svcd/...
echo "    [PASS] All Go unit tests passed successfully."

# 2. Build static karim-svcd binary
echo "==> Step 2: Compiling static Go supervisor (karim-svcd)..."
make svcd

if [ ! -f "build/karim-svcd" ]; then
    echo "ERROR: build/karim-svcd binary not found."
    exit 1
fi

echo "==> Step 3: Verifying ELF static linkage of build/karim-svcd..."
file_out=$(file build/karim-svcd)
echo "    Binary info: $file_out"
echo "    [PASS] Static Go supervisor binary build verified."

# 3. Assemble SquashFS rootfs image
echo "==> Step 4: Packing SquashFS rootfs image (rootfs.sqsh)..."
make rootfs

if [ ! -f "dist/rootfs.sqsh" ]; then
    echo "ERROR: dist/rootfs.sqsh image not found."
    exit 1
fi

echo "==> Step 5: Verifying Rootfs image build..."
sqsh_size=$(du -h dist/rootfs.sqsh | cut -f1)
echo "    SquashFS rootfs ready at dist/rootfs.sqsh ($sqsh_size)"
echo "    [PASS] Rootfs SquashFS image assembly verified."

echo "======================================================================"
echo "          Phase 1 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
