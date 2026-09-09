package svcd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasErr   bool
	}{
		{"32M", 32 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"512KB", 512 * 1024, false},
		{"1048576", 1048576, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := parseMemoryLimit(tt.input)
		if (err != nil) != tt.hasErr {
			t.Errorf("parseMemoryLimit(%q) error = %v, expected error %v", tt.input, err, tt.hasErr)
		}
		if got != tt.expected {
			t.Errorf("parseMemoryLimit(%q) = %d, expected %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseCPUQuota(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"50%", 50.0},
		{"100%", 100.0},
		{"25.5", 25.5},
	}

	for _, tt := range tests {
		got, err := parseCPUQuota(tt.input)
		if err != nil {
			t.Errorf("parseCPUQuota(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.expected {
			t.Errorf("parseCPUQuota(%q) = %f, expected %f", tt.input, got, tt.expected)
		}
	}
}

func TestParseServiceConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_service.toml")

	content := `[service]
name = "test_service"
exec = "/bin/echo"
args = ["hello", "world"]
env = ["TEST_VAR=1"]
after = ["network"]
memory_limit = "64M"
cpu_quota = "50%"
restart = "always"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	spec, err := ParseServiceConfig(configPath)
	if err != nil {
		t.Fatalf("ParseServiceConfig failed: %v", err)
	}

	if spec.Name != "test_service" {
		t.Errorf("expected name 'test_service', got %s", spec.Name)
	}
	if spec.Exec != "/bin/echo" {
		t.Errorf("expected exec '/bin/echo', got %s", spec.Exec)
	}
	if len(spec.Args) != 2 || spec.Args[0] != "hello" || spec.Args[1] != "world" {
		t.Errorf("unexpected args: %v", spec.Args)
	}
	if spec.MemoryLimit != 64*1024*1024 {
		t.Errorf("expected memory_limit 67108864, got %d", spec.MemoryLimit)
	}
	if spec.Restart != "always" {
		t.Errorf("expected restart 'always', got %s", spec.Restart)
	}
}
