package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEntropyParsing(t *testing.T) {
	tmpDir := t.TempDir()
	mockEntropyFile := filepath.Join(tmpDir, "entropy_avail")

	if err := os.WriteFile(mockEntropyFile, []byte("3584\n"), 0644); err != nil {
		t.Fatalf("failed writing mock entropy file: %v", err)
	}

	oldPath := procEntropyPath
	procEntropyPath = mockEntropyFile
	defer func() { procEntropyPath = oldPath }()

	val, err := GetEntropyAvailable()
	if err != nil {
		t.Fatalf("GetEntropyAvailable failed: %v", err)
	}
	if val != 3584 {
		t.Errorf("expected entropy 3584, got %d", val)
	}
}

func TestDebugFlagParsing(t *testing.T) {
	tmpDir := t.TempDir()
	mockCmdline := filepath.Join(tmpDir, "cmdline")

	oldPath := procCmdlinePath
	procCmdlinePath = mockCmdline
	defer func() { procCmdlinePath = oldPath }()

	// Test non-debug cmdline
	if err := os.WriteFile(mockCmdline, []byte("console=ttyS0 quiet panic=0 init=/init\n"), 0644); err != nil {
		t.Fatalf("failed writing mock cmdline: %v", err)
	}
	if IsDebugEnabled() {
		t.Errorf("expected IsDebugEnabled to be false for non-debug cmdline")
	}

	// Test debug cmdline
	if err := os.WriteFile(mockCmdline, []byte("console=ttyS0 debug panic=0 init=/init karim.debug=1\n"), 0644); err != nil {
		t.Fatalf("failed writing mock cmdline: %v", err)
	}
	if !IsDebugEnabled() {
		t.Errorf("expected IsDebugEnabled to be true when karim.debug=1 is present")
	}
}

func TestGetSystemStatus(t *testing.T) {
	tmpDir := t.TempDir()
	mockEntropyFile := filepath.Join(tmpDir, "entropy_avail")
	mockCmdline := filepath.Join(tmpDir, "cmdline")

	_ = os.WriteFile(mockEntropyFile, []byte("4096\n"), 0644)
	_ = os.WriteFile(mockCmdline, []byte("console=ttyS0 karim.debug=1\n"), 0644)

	oldEntropy := procEntropyPath
	oldCmdline := procCmdlinePath
	procEntropyPath = mockEntropyFile
	procCmdlinePath = mockCmdline
	defer func() {
		procEntropyPath = oldEntropy
		procCmdlinePath = oldCmdline
	}()

	status, err := GetSystemStatus()
	if err != nil {
		t.Fatalf("GetSystemStatus failed: %v", err)
	}

	if status.EntropyAvail != 4096 {
		t.Errorf("expected entropy 4096, got %d", status.EntropyAvail)
	}
	if !status.DebugMode {
		t.Errorf("expected DebugMode true")
	}
	if status.CurrentTime.IsZero() {
		t.Errorf("expected non-zero CurrentTime")
	}
}
