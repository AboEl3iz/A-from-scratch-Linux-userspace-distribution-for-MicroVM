package pkgd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Supported layer types
const (
	LayerTypeOCIArchive = "oci-archive"
	LayerTypeTarball    = "tarball"
	LayerTypeDirectory  = "directory"
	LayerTypeBinary     = "binary"
)

// LayerSpec defines an individual layer definition in config/layers.toml.
type LayerSpec struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	TargetDir string `json:"target_dir,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

// LayerConfig represents the top-level layers configuration structure.
type LayerConfig struct {
	Layers []LayerSpec `json:"layers"`
}

// LoadLayerConfig parses TOML or JSON layer definition file from disk.
func LoadLayerConfig(filePath string) (*LayerConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &LayerConfig{Layers: []LayerSpec{}}, nil
		}
		return nil, fmt.Errorf("failed reading layer config %s: %w", filePath, err)
	}

	// 1. Try JSON parsing if file ends with .json or starts with '{'
	if strings.HasSuffix(filePath, ".json") || strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		var cfg LayerConfig
		if err := json.Unmarshal(data, &cfg); err == nil {
			return &cfg, nil
		}
	}

	// 2. Parse TOML format line-by-line (zero-dependency)
	return parseLayerTOML(filePath)
}

func parseLayerTOML(filePath string) (*LayerConfig, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &LayerConfig{Layers: []LayerSpec{}}
	scanner := bufio.NewScanner(f)

	var current *LayerSpec

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "[[layer]]" || line == "[layer]" {
			if current != nil {
				cfg.Layers = append(cfg.Layers, *current)
			}
			current = &LayerSpec{Type: LayerTypeOCIArchive}
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := unquote(strings.TrimSpace(parts[1]))

		if current == nil {
			current = &LayerSpec{Type: LayerTypeOCIArchive}
		}

		switch key {
		case "name":
			current.Name = val
		case "type":
			current.Type = val
		case "path":
			current.Path = val
		case "target_dir":
			current.TargetDir = val
		case "sha256":
			current.SHA256 = val
		}
	}

	if current != nil && current.Name != "" {
		cfg.Layers = append(cfg.Layers, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	return cfg, nil
}

// SaveLayerConfig writes LayerConfig as formatted TOML/JSON to filePath.
func SaveLayerConfig(cfg *LayerConfig, filePath string) error {
	if cfg == nil {
		cfg = &LayerConfig{}
	}

	if strings.HasSuffix(filePath, ".json") {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filePath, data, 0644)
	}

	// Write TOML format
	var sb strings.Builder
	sb.WriteString("# Karim MicroVM OS — Layer Specification Config\n\n")

	for _, spec := range cfg.Layers {
		sb.WriteString("[[layer]]\n")
		sb.WriteString(fmt.Sprintf("name = %q\n", spec.Name))
		sb.WriteString(fmt.Sprintf("type = %q\n", spec.Type))
		sb.WriteString(fmt.Sprintf("path = %q\n", spec.Path))
		if spec.TargetDir != "" {
			sb.WriteString(fmt.Sprintf("target_dir = %q\n", spec.TargetDir))
		}
		if spec.SHA256 != "" {
			sb.WriteString(fmt.Sprintf("sha256 = %q\n", spec.SHA256))
		}
		sb.WriteString("\n")
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

func unquote(s string) string {
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`)) {
		return s[1 : len(s)-1]
	}
	return s
}
