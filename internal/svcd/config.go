package svcd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ServiceSpec defines the configuration structure for a service managed by karim-svcd.
type ServiceSpec struct {
	Name             string   `json:"name"`
	Exec             string   `json:"exec"`
	Args             []string `json:"args"`
	Env              []string `json:"env"`
	Directory        string   `json:"directory"`
	After            []string `json:"after"`
	MemoryLimit      int64    `json:"memory_limit"` // Bytes (0 means unlimited)
	CPUQuota         float64  `json:"cpu_quota"`    // Percentage (e.g. 50.0 for 50%)
	Restart          string   `json:"restart"`      // "always", "on-failure", "never"
	SeccompProfile   string   `json:"seccomp_profile"`
	CapabilitiesAdd  []string `json:"capabilities_add"`
	CapabilitiesDrop []string `json:"capabilities_drop"`
	NoNewPrivs       bool     `json:"no_new_privs"`
}

// ParseServiceConfig parses a single TOML service configuration file.
func ParseServiceConfig(path string) (*ServiceSpec, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}
	defer file.Close()

	spec := &ServiceSpec{
		Restart:        "on-failure",
		SeccompProfile: "app-default",
		NoNewPrivs:     true,
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	currentSection := ""

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip inline comment if present outside quotes
		if idx := strings.Index(line, "#"); idx >= 0 {
			inQuotes := false
			quoteChar := byte(0)
			for i := 0; i < idx; i++ {
				if (line[i] == '"' || line[i] == '\'') && (i == 0 || line[i-1] != '\\') {
					if !inQuotes {
						inQuotes = true
						quoteChar = line[i]
					} else if line[i] == quoteChar {
						inQuotes = false
					}
				}
			}
			if !inQuotes {
				line = strings.TrimSpace(line[:idx])
				if line == "" {
					continue
				}
			}
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			secName := strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			currentSection = secName
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if currentSection == "env" || currentSection == "environment" {
			spec.Env = append(spec.Env, fmt.Sprintf("%s=%s", key, unquote(val)))
			continue
		}

		switch strings.ToLower(key) {
		case "name":
			spec.Name = unquote(val)
		case "exec":
			spec.Exec = unquote(val)
		case "args":
			spec.Args = parseArray(val)
		case "env", "environment":
			spec.Env = append(spec.Env, parseArray(val)...)
		case "directory":
			spec.Directory = unquote(val)
		case "after":
			spec.After = parseArray(val)
		case "memory_limit":
			bytes, err := parseMemoryLimit(unquote(val))
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid memory_limit %q: %w", lineNum, val, err)
			}
			spec.MemoryLimit = bytes
		case "cpu_quota":
			quota, err := parseCPUQuota(unquote(val))
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid cpu_quota %q: %w", lineNum, val, err)
			}
			spec.CPUQuota = quota
		case "restart":
			spec.Restart = unquote(val)
		case "seccomp_profile":
			spec.SeccompProfile = unquote(val)
		case "capabilities_add", "capabilities.add":
			spec.CapabilitiesAdd = parseArray(val)
		case "capabilities_drop", "capabilities.drop":
			spec.CapabilitiesDrop = parseArray(val)
		case "no_new_privs":
			spec.NoNewPrivs = parseBool(val)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", path, err)
	}

	if spec.Exec == "" {
		if lineNum == 0 {
			return nil, fmt.Errorf("service file %s is empty", path)
		}
		if spec.Name == "" {
			base := filepath.Base(path)
			spec.Name = strings.TrimSuffix(base, filepath.Ext(base))
		}
		return nil, fmt.Errorf("service %s missing mandatory 'exec' binary path", spec.Name)
	}

	if spec.Name == "" {
		base := filepath.Base(path)
		spec.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return spec, nil
}

// LoadServiceDir scans a directory for *.toml files and returns all parsed ServiceSpecs.
func LoadServiceDir(dir string) ([]*ServiceSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read service dir %s: %w", dir, err)
	}

	var services []*ServiceSpec
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		spec, err := ParseServiceConfig(path)
		if err != nil {
			if strings.Contains(err.Error(), "is empty") {
				continue
			}
			return nil, fmt.Errorf("error parsing %s: %w", path, err)
		}
		services = append(services, spec)
	}

	return services, nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`)) {
		return s[1 : len(s)-1]
	}
	return s
}

func parseArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	content := strings.TrimSpace(s[1 : len(s)-1])
	if content == "" {
		return []string{}
	}

	var result []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	escaped := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inQuotes {
			current.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '"' || ch == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuotes = false
			}
			current.WriteByte(ch)
			continue
		}

		if ch == ',' && !inQuotes {
			item := unquote(strings.TrimSpace(current.String()))
			if item != "" {
				result = append(result, item)
			}
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		item := unquote(strings.TrimSpace(current.String()))
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

func parseMemoryLimit(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	multiplier := int64(1)
	upper := strings.ToUpper(s)

	if strings.HasSuffix(upper, "G") || strings.HasSuffix(upper, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimRight(upper, "GB")
	} else if strings.HasSuffix(upper, "M") || strings.HasSuffix(upper, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimRight(upper, "MB")
	} else if strings.HasSuffix(upper, "K") || strings.HasSuffix(upper, "KB") {
		multiplier = 1024
		s = strings.TrimRight(upper, "KB")
	}

	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return val * multiplier, nil
}

func parseCPUQuota(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.0, nil
	}
	s = strings.TrimSuffix(s, "%")
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func parseBool(s string) bool {
	s = strings.ToLower(unquote(strings.TrimSpace(s)))
	return s == "true" || s == "1" || s == "yes"
}
