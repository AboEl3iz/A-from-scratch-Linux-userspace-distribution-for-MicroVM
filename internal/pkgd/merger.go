package pkgd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UnpackOCIArchive extracts all ordered layer deltas from a Docker/OCI tarball into targetRoot.
func UnpackOCIArchive(tarPath, targetRoot string) error {
	info, err := InspectOCIArchive(tarPath)
	if err != nil {
		return fmt.Errorf("failed inspecting OCI archive: %w", err)
	}

	if len(info.LayerPaths) == 0 {
		return fmt.Errorf("no layers found in OCI archive %s", tarPath)
	}

	// Create map of target layer paths
	layerSet := make(map[string]bool)
	for _, l := range info.LayerPaths {
		layerSet[filepath.Clean(l)] = true
	}

	layerData := make(map[string][]byte)

	// Open tarball and cache layer blobs
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed opening OCI tar %s: %w", tarPath, err)
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(tarPath, ".gz") || strings.HasSuffix(tarPath, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("failed uncompressing gzip tar: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed reading archive tar entry: %w", err)
		}

		cleanName := filepath.Clean(hdr.Name)
		if layerSet[cleanName] {
			data, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("failed reading layer tar payload %s: %w", cleanName, err)
			}
			layerData[cleanName] = data
		}
	}

	// Extract layers in strict sequential order
	for idx, layerPath := range info.LayerPaths {
		cleanPath := filepath.Clean(layerPath)
		data, ok := layerData[cleanPath]
		if !ok {
			return fmt.Errorf("layer payload %s missing from archive tarball", layerPath)
		}

		var layerReader io.Reader = bytes.NewReader(data)
		if (len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b) || strings.HasSuffix(layerPath, ".gz") || strings.HasSuffix(layerPath, ".tgz") {
			gz, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("failed uncompressing layer tar %s: %w", layerPath, err)
			}
			defer gz.Close()
			layerReader = gz
		}

		if err := ExtractLayerTarball(layerReader, targetRoot); err != nil {
			return fmt.Errorf("failed extracting layer %d (%s): %w", idx, layerPath, err)
		}
	}

	return nil
}

// MergeLayerConfig applies all declared layers in LayerConfig into targetRoot.
func MergeLayerConfig(cfg *LayerConfig, baseDir, targetRoot string) error {
	if cfg == nil || len(cfg.Layers) == 0 {
		return nil
	}

	for idx, spec := range cfg.Layers {
		layerPath := spec.Path
		if !filepath.IsAbs(layerPath) {
			layerPath = filepath.Join(baseDir, spec.Path)
		}

		if _, err := os.Stat(layerPath); err != nil {
			return fmt.Errorf("layer %d (%s) source path missing: %w", idx, spec.Name, err)
		}

		targetDir := targetRoot
		if spec.TargetDir != "" {
			targetDir = filepath.Join(targetRoot, spec.TargetDir)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed creating layer target dir: %w", err)
		}

		switch spec.Type {
		case LayerTypeOCIArchive:
			if err := UnpackOCIArchive(layerPath, targetDir); err != nil {
				return fmt.Errorf("failed merging OCI archive layer %s: %w", spec.Name, err)
			}

		case LayerTypeTarball:
			f, err := os.Open(layerPath)
			if err != nil {
				return fmt.Errorf("failed opening tarball layer %s: %w", spec.Name, err)
			}
			var r io.Reader = f
			if strings.HasSuffix(layerPath, ".gz") || strings.HasSuffix(layerPath, ".tgz") {
				gz, err := gzip.NewReader(f)
				if err != nil {
					f.Close()
					return fmt.Errorf("failed uncompressing tarball %s: %w", spec.Name, err)
				}
				defer gz.Close()
				r = gz
			}
			err = ExtractLayerTarball(r, targetDir)
			f.Close()
			if err != nil {
				return fmt.Errorf("failed extracting tarball layer %s: %w", spec.Name, err)
			}

		case LayerTypeDirectory:
			if err := copyDirectoryTree(layerPath, targetDir); err != nil {
				return fmt.Errorf("failed copying directory layer %s: %w", spec.Name, err)
			}

		case LayerTypeBinary:
			destFile := filepath.Join(targetDir, filepath.Base(layerPath))
			if err := copyExecutableFile(layerPath, destFile); err != nil {
				return fmt.Errorf("failed copying binary layer %s: %w", spec.Name, err)
			}

		default:
			return fmt.Errorf("unsupported layer type %q for layer %s", spec.Type, spec.Name)
		}
	}

	return nil
}

func copyDirectoryTree(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		targetPath := filepath.Join(destDir, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return copyExecutableFile(path, targetPath)
	})
}

func copyExecutableFile(srcPath, destPath string) error {
	sf, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer sf.Close()

	info, err := sf.Stat()
	if err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if info.Mode()&0111 != 0 {
		mode = 0755
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	df, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer df.Close()

	if _, err := io.Copy(df, sf); err != nil {
		return err
	}

	return nil
}
