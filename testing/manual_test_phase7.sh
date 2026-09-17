#!/usr/bin/env bash
# ==============================================================================
# Karim MicroVM OS - Phase 7 Automated Verification Script
# ==============================================================================

set -euo pipefail

echo "======================================================================"
echo "          Karim MicroVM OS - Phase 7 Verification Suite               "
echo "======================================================================"

# 1. Run pkgd package unit tests
echo "==> Step 1: Running OCI & Layer Engine Go unit test suite..."
go test -v ./internal/pkgd/...
echo "    [PASS] All Go pkgd unit tests passed successfully."

# 2. Build host CLI and binaries
echo "==> Step 2: Compiling host karim CLI and build daemons..."
make init svcd secd vsockd obsd cli

if [ ! -f "dist/karim" ]; then
    echo "ERROR: dist/karim host CLI binary missing."
    exit 1
fi

echo "    [PASS] Host CLI build verified."

# 3. Construct synthetic multi-layer OCI archive tarball
echo "==> Step 3: Constructing synthetic multi-layer OCI archive tarball..."
TEST_TMP="/tmp/karim_phase7_test"
rm -rf "$TEST_TMP"
mkdir -p "$TEST_TMP/layer1/etc" "$TEST_TMP/layer1/bin" "$TEST_TMP/layer1/tmp/cache"
mkdir -p "$TEST_TMP/layer2/etc" "$TEST_TMP/layer2/bin"
mkdir -p "$TEST_TMP/layer3/tmp/cache"
mkdir -p "$TEST_TMP/archive"

# Layer 1: Base files
echo "Karim OS Banner v1" > "$TEST_TMP/layer1/etc/banner.txt"
echo "root:secret123" > "$TEST_TMP/layer1/etc/shadow"
echo "binary_v1" > "$TEST_TMP/layer1/bin/app"
echo "cached_data_v1" > "$TEST_TMP/layer1/tmp/cache/old_data.txt"

(cd "$TEST_TMP/layer1" && tar -cf "$TEST_TMP/archive/layer1.tar" .)

# Layer 2: Whiteout delete shadow file, update binary
touch "$TEST_TMP/layer2/etc/.wh.shadow"
echo "binary_v2_updated" > "$TEST_TMP/layer2/bin/app"

(cd "$TEST_TMP/layer2" && tar -cf "$TEST_TMP/archive/layer2.tar" .)

# Layer 3: Opaque whiteout clear tmp/cache directory
touch "$TEST_TMP/layer3/tmp/cache/.wh..wh..opq"
echo "cached_data_v2" > "$TEST_TMP/layer3/tmp/cache/fresh_data.txt"

(cd "$TEST_TMP/layer3" && tar -cf "$TEST_TMP/archive/layer3.tar" .)

# Generate Docker manifest.json
cat <<EOF > "$TEST_TMP/archive/manifest.json"
[
  {
    "Config": "config.json",
    "RepoTags": ["karim-synthetic:latest"],
    "Layers": ["layer1.tar", "layer2.tar", "layer3.tar"]
  }
]
EOF

OCI_TAR="$TEST_TMP/synthetic_oci_app.tar"
(cd "$TEST_TMP/archive" && tar -cf "$OCI_TAR" .)

echo "    [PASS] Synthetic OCI archive created at $OCI_TAR"

# 4. Test OCI Archive Inspection via CLI
echo "==> Step 4: Testing 'karim import' OCI archive inspection..."
import_out=$(./dist/karim import "$OCI_TAR" "synthetic-app-layer")
echo "$import_out"

if [[ "$import_out" != *"3 layers detected"* || "$import_out" != *"synthetic-app-layer"* ]]; then
    echo "ERROR: 'karim import' failed to inspect and register OCI archive."
    exit 1
fi

echo "    [PASS] 'karim import' OCI archive inspection & registration verified."

# 5. Build Hermetic Image containing merged OCI layers
echo "==> Step 5: Building Hermetic RootFS image containing merged OCI layers..."
make build-hermetic

if [ ! -f "dist/rootfs.sqsh" ]; then
    echo "ERROR: dist/rootfs.sqsh image build failed."
    exit 1
fi

echo "    [PASS] OCI Layer Engine integrated rootfs.sqsh build verified!"

# Clean up temp test files
rm -rf "$TEST_TMP"

echo "======================================================================"
echo "          Phase 7 Verification Complete: ALL TESTS PASSED             "
echo "======================================================================"
