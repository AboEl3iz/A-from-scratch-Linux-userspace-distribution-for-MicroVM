package svcd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCGroupManager_CreateServiceCGroup(t *testing.T) {
	tmpDir := t.TempDir()
	cgm := &CGroupManager{basePath: tmpDir}

	spec := &ServiceSpec{
		Name:        "test_cgroup_app",
		MemoryLimit: 32 * 1024 * 1024,
		CPUQuota:    50.0,
	}

	path, err := cgm.CreateServiceCGroup(spec)
	if err != nil {
		t.Fatalf("CreateServiceCGroup failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "test_cgroup_app")
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("cgroup directory %s was not created", path)
	}
}
