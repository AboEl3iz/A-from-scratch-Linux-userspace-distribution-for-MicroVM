package pkgd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DockerManifestEntry represents an entry in Docker archive manifest.json.
type DockerManifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// OCISpecManifest represents OCI image manifest JSON.
type OCISpecManifest struct {
	SchemaVersion int `json:"schemaVersion"`
	Layers        []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int64  `json:"size"`
	} `json:"layers"`
}

// OCIArchiveInfo contains metadata and ordered layer path list extracted from an OCI/Docker archive.
type OCIArchiveInfo struct {
	RepoTags   []string
	LayerPaths []string
}

const (
	WhiteoutPrefix = ".wh."
	OpaqueWhiteout = ".wh..wh..opq"
)

// InspectOCIArchive inspects a tarball archive to read Docker/OCI manifest layer ordering.
func InspectOCIArchive(tarPath string) (*OCIArchiveInfo, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, fmt.Errorf("failed opening OCI tar %s: %w", tarPath, err)
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(tarPath, ".gz") || strings.HasSuffix(tarPath, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed uncompressing gzip tar %s: %w", tarPath, err)
		}
		defer gz.Close()
		reader = gz
	}

	tr := tar.NewReader(reader)
	info := &OCIArchiveInfo{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read error in %s: %w", tarPath, err)
		}

		cleanName := filepath.Clean(hdr.Name)

		// 1. Docker format manifest.json
		if cleanName == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed reading manifest.json: %w", err)
			}

			var dockerManifest []DockerManifestEntry
			if err := json.Unmarshal(data, &dockerManifest); err == nil && len(dockerManifest) > 0 {
				info.RepoTags = dockerManifest[0].RepoTags
				info.LayerPaths = dockerManifest[0].Layers
				return info, nil
			}
		}
	}

	return nil, fmt.Errorf("no recognized manifest.json found in OCI archive %s", tarPath)
}

// ExtractLayerTarball extracts a single layer tarball into destDir with OCI whiteout resolution.
func ExtractLayerTarball(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading layer tar entry: %w", err)
		}

		cleanPath := filepath.Clean(hdr.Name)
		dirName := filepath.Dir(cleanPath)
		baseName := filepath.Base(cleanPath)

		// 1. Handle OCI Opaque Whiteout (.wh..wh..opq)
		if baseName == OpaqueWhiteout {
			targetDir := filepath.Join(destDir, dirName)
			if err := clearDirectoryContents(targetDir); err != nil {
				return fmt.Errorf("opaque whiteout failed for %s: %w", targetDir, err)
			}
			continue
		}

		// 2. Handle OCI File Whiteout (.wh.<filename>)
		if strings.HasPrefix(baseName, WhiteoutPrefix) {
			deletedName := strings.TrimPrefix(baseName, WhiteoutPrefix)
			targetFile := filepath.Join(destDir, dirName, deletedName)
			_ = os.RemoveAll(targetFile)
			continue
		}

		// 3. Regular File / Directory / Symlink Extraction
		targetPath := filepath.Join(destDir, cleanPath)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed creating dir %s: %w", targetPath, err)
			}

		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed creating parent dir for %s: %w", targetPath, err)
			}

			mode := os.FileMode(0644)
			if hdr.Mode&0111 != 0 {
				mode = 0755
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("failed creating file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed writing data to %s: %w", targetPath, err)
			}
			outFile.Close()

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed creating parent dir for symlink %s: %w", targetPath, err)
			}
			_ = os.Remove(targetPath)
			linkVal := hdr.Linkname
			if !strings.HasPrefix(linkVal, "/") && !strings.HasPrefix(linkVal, ".") {
				linkVal = "/" + linkVal
			}
			if err := os.Symlink(linkVal, targetPath); err != nil {
				return fmt.Errorf("failed creating symlink %s -> %s: %w", targetPath, linkVal, err)
			}

		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed creating parent dir for hardlink %s: %w", targetPath, err)
			}
			_ = os.Remove(targetPath)
			linkTarget := filepath.Join(destDir, hdr.Linkname)
			if err := os.Link(linkTarget, targetPath); err != nil {
				linkVal := hdr.Linkname
				if !strings.HasPrefix(linkVal, "/") && !strings.HasPrefix(linkVal, ".") {
					linkVal = "/" + linkVal
				}
				_ = os.Symlink(linkVal, targetPath)
			}
		}
	}

	return nil
}

func clearDirectoryContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}

	return nil
}
