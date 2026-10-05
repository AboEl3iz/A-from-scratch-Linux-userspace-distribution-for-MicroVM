package svcd

import (
	"encoding/json"
	"strings"
	"testing"

	"karim-microvm-os/internal/pkgd"
)

func TestParseAndPrepareOCIBundle(t *testing.T) {
	memLimit := int64(67108864) // 64MB
	spec := &pkgd.Spec{
		Process: &pkgd.Process{
			Args: []string{"/bin/sh", "-c", "echo hello from OCI bundle"},
			Env:  []string{"PATH=/bin:/usr/bin", "FOO=BAR"},
			Cwd:  "/",
		},
		Linux: &pkgd.Linux{
			Seccomp: &pkgd.LinuxSeccomp{
				ProfileName: "app-default",
			},
			Resources: &pkgd.LinuxResources{
				Memory: &pkgd.LinuxMemory{
					Limit: &memLimit,
				},
			},
		},
	}

	bundle := pkgd.OCIContainerBundle{
		BundleID:  "test-bundle-1",
		Spec:      spec,
		LowerDirs: []string{"/tmp"},
	}

	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("failed to marshal bundle: %v", err)
	}

	svcSpec, err := ParseAndPrepareOCIBundle(string(bundleJSON))
	if err != nil {
		t.Fatalf("ParseAndPrepareOCIBundle failed: %v", err)
	}

	if svcSpec.Name != "test-bundle-1" {
		t.Errorf("expected spec name test-bundle-1, got %s", svcSpec.Name)
	}
	if svcSpec.Exec != "/bin/sh" {
		t.Errorf("expected exec /bin/sh, got %s", svcSpec.Exec)
	}
	if len(svcSpec.Args) != 2 || svcSpec.Args[0] != "-c" {
		t.Errorf("expected args [-c echo...], got %v", svcSpec.Args)
	}
	if svcSpec.MemoryLimit != memLimit {
		t.Errorf("expected memory limit %d, got %d", memLimit, svcSpec.MemoryLimit)
	}
	if !strings.HasSuffix(svcSpec.RootDir, "test-bundle-1/rootfs") {
		t.Errorf("expected rootDir ending with test-bundle-1/rootfs, got %s", svcSpec.RootDir)
	}
}
