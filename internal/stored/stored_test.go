package stored

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOverlayOptions(t *testing.T) {
	opts := BuildOverlayOptions("/mnt/lower", "/mnt/upper", "/mnt/work")
	expected := "lowerdir=/mnt/lower,upperdir=/mnt/upper,workdir=/mnt/work"
	if opts != expected {
		t.Errorf("Expected %q, got %q", expected, opts)
	}
}

func TestPrepareOverlayDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	upper, work, err := PrepareOverlayDirectories(tmpDir)
	if err != nil {
		t.Fatalf("PrepareOverlayDirectories failed: %v", err)
	}

	if upper != filepath.Join(tmpDir, "upper") {
		t.Errorf("Unexpected upper dir: %s", upper)
	}
	if work != filepath.Join(tmpDir, "work") {
		t.Errorf("Unexpected work dir: %s", work)
	}

	if info, err := os.Stat(upper); err != nil || !info.IsDir() {
		t.Errorf("Upper dir was not created properly: %v", err)
	}
	if info, err := os.Stat(work); err != nil || !info.IsDir() {
		t.Errorf("Work dir was not created properly: %v", err)
	}
}

func TestIsMountPoint(t *testing.T) {
	// Root or /proc should be mount points on any Linux host
	mounted, err := IsMountPoint("/")
	if err != nil {
		t.Fatalf("IsMountPoint('/') returned error: %v", err)
	}
	if !mounted {
		t.Logf("Notice: '/' reported not mounted via /proc/mounts (container/chroot environment)")
	}

	// Non-existent or newly created tmp dir should not be a mount point
	tmpDir := t.TempDir()
	mountedTmp, err := IsMountPoint(tmpDir)
	if err != nil {
		t.Fatalf("IsMountPoint(tmpDir) error: %v", err)
	}
	if mountedTmp {
		t.Errorf("Expected %s not to be a mount point", tmpDir)
	}
}

func TestInvalidOverlayConfig(t *testing.T) {
	cfg := OverlayConfig{
		LowerDir: "/lower",
		// missing upper, work, target
	}
	err := MountOverlay(cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid OverlayConfig") {
		t.Errorf("Expected invalid OverlayConfig error, got: %v", err)
	}
}
