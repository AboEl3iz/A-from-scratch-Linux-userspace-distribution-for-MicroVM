package pkgd

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTarArchive(files map[string]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Write whiteout markers first
	for name, content := range files {
		if strings.Contains(name, ".wh.") {
			hdr := &tar.Header{
				Name:     name,
				Mode:     0644,
				Size:     int64(len(content)),
				Typeflag: tar.TypeReg,
			}
			_ = tw.WriteHeader(hdr)
			_, _ = tw.Write([]byte(content))
		}
	}

	// Write regular files
	for name, content := range files {
		if !strings.Contains(name, ".wh.") {
			hdr := &tar.Header{
				Name:     name,
				Mode:     0644,
				Size:     int64(len(content)),
				Typeflag: tar.TypeReg,
			}
			_ = tw.WriteHeader(hdr)
			_, _ = tw.Write([]byte(content))
		}
	}
	tw.Close()
	return buf.Bytes()
}

func TestOCIWhiteoutExtraction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pkgd-test-*")
	if err != nil {
		t.Fatalf("Failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetRoot := filepath.Join(tmpDir, "rootfs")

	// Layer 1: Base files
	layer1Files := map[string]string{
		"etc/config.txt":  "v1 config",
		"etc/shadow":      "root:secret",
		"var/log/app.log": "init log",
	}
	l1Data := createTarArchive(layer1Files)
	if err := ExtractLayerTarball(bytes.NewReader(l1Data), targetRoot); err != nil {
		t.Fatalf("Layer 1 extraction failed: %v", err)
	}

	// Verify Layer 1 state
	if _, err := os.Stat(filepath.Join(targetRoot, "etc/shadow")); err != nil {
		t.Fatalf("etc/shadow missing after layer 1")
	}

	// Layer 2: Deletes etc/shadow using whiteout .wh.shadow, modifies etc/config.txt
	layer2Files := map[string]string{
		"etc/.wh.shadow": "",
		"etc/config.txt": "v2 config updated",
		"usr/bin/tool":   "binary data",
	}
	l2Data := createTarArchive(layer2Files)
	if err := ExtractLayerTarball(bytes.NewReader(l2Data), targetRoot); err != nil {
		t.Fatalf("Layer 2 extraction failed: %v", err)
	}

	// Assert etc/shadow was deleted by whiteout
	if _, err := os.Stat(filepath.Join(targetRoot, "etc/shadow")); !os.IsNotExist(err) {
		t.Fatalf("etc/shadow still exists after whiteout deletion!")
	}

	// Assert etc/config.txt was updated to v2
	data, _ := os.ReadFile(filepath.Join(targetRoot, "etc/config.txt"))
	if string(data) != "v2 config updated" {
		t.Fatalf("etc/config.txt content mismatch: %s", string(data))
	}

	// Layer 3: Opaque whiteout clearing var/log
	layer3Files := map[string]string{
		"var/log/.wh..wh..opq": "",
		"var/log/new.log":      "new log",
	}
	l3Data := createTarArchive(layer3Files)
	if err := ExtractLayerTarball(bytes.NewReader(l3Data), targetRoot); err != nil {
		t.Fatalf("Layer 3 extraction failed: %v", err)
	}

	// Assert old var/log/app.log was cleared by opaque whiteout
	if _, err := os.Stat(filepath.Join(targetRoot, "var/log/app.log")); !os.IsNotExist(err) {
		t.Fatalf("var/log/app.log still exists after opaque whiteout!")
	}

	// Assert new log exists
	if _, err := os.Stat(filepath.Join(targetRoot, "var/log/new.log")); err != nil {
		t.Fatalf("var/log/new.log missing after layer 3")
	}

	t.Log("OCI whiteout extraction test PASSED successfully")
}

func TestLayerConfigLoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pkgd-cfg-*")
	if err != nil {
		t.Fatalf("Failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "layers.toml")
	cfg := &LayerConfig{
		Layers: []LayerSpec{
			{Name: "base-alpine", Type: LayerTypeOCIArchive, Path: "layers/alpine.tar"},
			{Name: "custom-tool", Type: LayerTypeBinary, Path: "bin/tool", TargetDir: "usr/bin"},
		},
	}

	if err := SaveLayerConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveLayerConfig failed: %v", err)
	}

	loaded, err := LoadLayerConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadLayerConfig failed: %v", err)
	}

	if len(loaded.Layers) != 2 {
		t.Fatalf("Expected 2 layers in config, got %d", len(loaded.Layers))
	}

	if loaded.Layers[0].Name != "base-alpine" || loaded.Layers[1].TargetDir != "usr/bin" {
		t.Fatalf("LayerSpec fields mismatch after load")
	}

	t.Log("LayerConfig load/save test PASSED")
}
