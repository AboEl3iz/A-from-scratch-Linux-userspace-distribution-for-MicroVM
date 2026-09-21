package builder

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCPIOPackingReproducibility(t *testing.T) {
	// Create mock directory structure with files out of alphabetical order
	tmpDir, err := os.MkdirTemp("", "cpio-test-*")
	if err != nil {
		t.Fatalf("Failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"z_file.txt":        "content Z",
		"a_file.txt":        "content A",
		"sub/m_file.txt":    "content M",
		"sub/a_subfile.txt": "content A sub",
	}

	for p, content := range files {
		fullPath := filepath.Join(tmpDir, p)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed creating dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed writing test file: %v", err)
		}
	}

	// Pack run 1
	var buf1 bytes.Buffer
	fixedTime := time.Unix(0, 0)
	if err := PackCPIOInitramfs(tmpDir, &buf1, fixedTime); err != nil {
		t.Fatalf("PackCPIOInitramfs run 1 failed: %v", err)
	}

	// Sleep to ensure time difference if any timestamp leaked
	time.Sleep(10 * time.Millisecond)

	// Pack run 2
	var buf2 bytes.Buffer
	if err := PackCPIOInitramfs(tmpDir, &buf2, fixedTime); err != nil {
		t.Fatalf("PackCPIOInitramfs run 2 failed: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Fatalf("CPIO archives differ between run 1 (%d bytes) and run 2 (%d bytes)",
			buf1.Len(), buf2.Len())
	}

	t.Logf("CPIO pack succeeded: %d bytes (byte-for-byte identical)", buf1.Len())
}

func TestManifestComparison(t *testing.T) {
	m1 := &BuildManifest{
		ProjectName: "karim-microvm-os",
		OutputArtifacts: map[string]ArtifactChecksum{
			"initramfs.cpio": {Name: "initramfs.cpio", SHA256: "abc123def456"},
			"rootfs.sqsh":    {Name: "rootfs.sqsh", SHA256: "789xyz012345"},
		},
	}

	m2 := &BuildManifest{
		ProjectName: "karim-microvm-os",
		OutputArtifacts: map[string]ArtifactChecksum{
			"initramfs.cpio": {Name: "initramfs.cpio", SHA256: "abc123def456"},
			"rootfs.sqsh":    {Name: "rootfs.sqsh", SHA256: "789xyz012345"},
		},
	}

	if err := CompareManifests(m1, m2); err != nil {
		t.Fatalf("CompareManifests expected success but failed: %v", err)
	}

	// Inject non-deterministic hash
	m2.OutputArtifacts["rootfs.sqsh"] = ArtifactChecksum{Name: "rootfs.sqsh", SHA256: "different_hash"}
	if err := CompareManifests(m1, m2); err == nil {
		t.Fatalf("CompareManifests expected failure on hash mismatch, but succeeded")
	}
}
