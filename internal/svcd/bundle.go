package svcd

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"karim-microvm-os/internal/pkgd"
	"karim-microvm-os/internal/stored"
)

// ParseAndPrepareOCIBundle parses an OCI bundle payload JSON, extracts tarball rootfs if present,
// sets up dynamic OverlayFS, and returns a fully configured ServiceSpec ready for secd execution.
func ParseAndPrepareOCIBundle(bundleJSON string) (*ServiceSpec, error) {
	var bundle pkgd.OCIContainerBundle
	if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OCI bundle JSON: %w", err)
	}

	if bundle.BundleID == "" {
		return nil, fmt.Errorf("bundle_id cannot be empty")
	}

	containerID := bundle.BundleID
	baseContainerDir := "/run/karim/containers"
	if err := os.MkdirAll(baseContainerDir, 0755); err != nil {
		baseContainerDir = filepath.Join(os.TempDir(), "karim_containers")
		_ = os.MkdirAll(baseContainerDir, 0755)
	}

	targetRootfs := filepath.Join(baseContainerDir, containerID, "rootfs")
	lowerDirs := bundle.LowerDirs

	// If rootfs tarball bytes are provided, extract into layer directory
	if len(bundle.RootfsTar) > 0 {
		layerDir := filepath.Join(baseContainerDir, containerID, "extracted")
		if err := os.MkdirAll(layerDir, 0755); err != nil {
			return nil, fmt.Errorf("failed creating layer dir %s: %w", layerDir, err)
		}
		if err := extractTarBytes(bundle.RootfsTar, layerDir); err != nil {
			return nil, fmt.Errorf("failed extracting rootfs tarball for container %s: %w", containerID, err)
		}
		lowerDirs = append(lowerDirs, layerDir)
	}

	// Fallback lowerdir if none specified
	if len(lowerDirs) == 0 {
		lowerDirs = []string{"/mnt/lower"}
	}

	// Mount dynamic OverlayFS for container
	if _, err := stored.MountDynamicOverlay(containerID, lowerDirs, targetRootfs); err != nil {
		return nil, fmt.Errorf("failed mounting dynamic overlay for bundle %s: %w", containerID, err)
	}

	// Build ServiceSpec from OCI Spec or defaults
	spec := &ServiceSpec{
		Name:             containerID,
		Exec:             "/bin/sh",
		Args:             nil,
		Env:              []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "TERM=xterm"},
		Directory:        "/",
		RootDir:          targetRootfs,
		Restart:          bundle.Restart,
		SeccompProfile:   "app-default",
		NoNewPrivs:       true,
	}
	if spec.Restart == "" {
		spec.Restart = "on-failure"
	}

	if bundle.Spec != nil {
		if bundle.Spec.Process != nil {
			if len(bundle.Spec.Process.Args) > 0 {
				spec.Exec = bundle.Spec.Process.Args[0]
				if len(bundle.Spec.Process.Args) > 1 {
					spec.Args = bundle.Spec.Process.Args[1:]
				}
			}
			if len(bundle.Spec.Process.Env) > 0 {
				spec.Env = bundle.Spec.Process.Env
			}
			if bundle.Spec.Process.Cwd != "" {
				spec.Directory = bundle.Spec.Process.Cwd
			}
			if bundle.Spec.Process.Capabilities != nil {
				spec.CapabilitiesAdd = bundle.Spec.Process.Capabilities.Permitted
				spec.CapabilitiesDrop = []string{"ALL"}
			}
		}
		if bundle.Spec.Linux != nil {
			if bundle.Spec.Linux.Seccomp != nil && bundle.Spec.Linux.Seccomp.ProfileName != "" {
				spec.SeccompProfile = bundle.Spec.Linux.Seccomp.ProfileName
			}
			if bundle.Spec.Linux.Resources != nil && bundle.Spec.Linux.Resources.Memory != nil && bundle.Spec.Linux.Resources.Memory.Limit != nil {
				spec.MemoryLimit = *bundle.Spec.Linux.Resources.Memory.Limit
			}
		}
	}

	return spec, nil
}

func extractTarBytes(tarBytes []byte, destDir string) error {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}
