package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// ArtifactChecksum records the SHA-256 identity and metadata of a file.
type ArtifactChecksum struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

// BuildManifest details all inputs, outputs, and hashes for a Karim OS build.
type BuildManifest struct {
	ProjectName       string                      `json:"project_name"`
	Version           string                      `json:"version"`
	BuildTimestamp    string                      `json:"build_timestamp"`
	Hermetic          bool                        `json:"hermetic"`
	SourceDateEpoch   int64                       `json:"source_date_epoch"`
	InputArtifacts    map[string]ArtifactChecksum `json:"input_artifacts"`
	OutputArtifacts   map[string]ArtifactChecksum `json:"output_artifacts"`
	CombinedBuildHash string                      `json:"combined_build_hash"`
}

// CalculateFileSHA256 computes the hexadecimal SHA-256 hash of a file.
func CalculateFileSHA256(filePath string) (string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("failed hashing %s: %w", filePath, err)
	}

	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// NewArtifactChecksum creates an ArtifactChecksum entry for a file on disk.
func NewArtifactChecksum(name, path string) (ArtifactChecksum, error) {
	hash, size, err := CalculateFileSHA256(path)
	if err != nil {
		return ArtifactChecksum{}, err
	}
	return ArtifactChecksum{
		Name:      name,
		Path:      path,
		SizeBytes: size,
		SHA256:    hash,
	}, nil
}

// CalculateCombinedBuildHash produces a single content-addressable SHA-256 digest of all inputs and outputs.
func (m *BuildManifest) CalculateCombinedBuildHash() string {
	h := sha256.New()

	var keys []string
	for k := range m.OutputArtifacts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(m.OutputArtifacts[k].SHA256))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// SaveManifest writes the BuildManifest as formatted JSON to destPath.
func SaveManifest(manifest *BuildManifest, destPath string) error {
	manifest.CombinedBuildHash = manifest.CalculateCombinedBuildHash()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed encoding build manifest: %w", err)
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed writing manifest to %s: %w", destPath, err)
	}
	return nil
}

// LoadManifest reads a BuildManifest from a JSON file.
func LoadManifest(srcPath string) (*BuildManifest, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading manifest %s: %w", srcPath, err)
	}
	var m BuildManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed parsing manifest JSON: %w", err)
	}
	return &m, nil
}

// CompareManifests verifies that two build manifests are byte-for-byte identical in output SHA-256 hashes.
func CompareManifests(m1, m2 *BuildManifest) error {
	if len(m1.OutputArtifacts) != len(m2.OutputArtifacts) {
		return fmt.Errorf("output artifact count mismatch (%d vs %d)", len(m1.OutputArtifacts), len(m2.OutputArtifacts))
	}

	for k, a1 := range m1.OutputArtifacts {
		a2, ok := m2.OutputArtifacts[k]
		if !ok {
			return fmt.Errorf("missing artifact %s in second build manifest", k)
		}
		if a1.SHA256 != a2.SHA256 {
			return fmt.Errorf("REPRODUCIBILITY FAILURE for artifact %s: build 1 hash = %s, build 2 hash = %s",
				k, a1.SHA256, a2.SHA256)
		}
	}

	return nil
}

// FormatFixedTimestamp returns a deterministic RFC3339 timestamp string for fixed epoch.
func FormatFixedTimestamp(epoch int64) string {
	return time.Unix(epoch, 0).UTC().Format(time.RFC3339)
}
