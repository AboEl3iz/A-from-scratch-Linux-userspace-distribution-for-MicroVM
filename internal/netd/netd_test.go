package netd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("eth0")
	if cfg.InterfaceName != "eth0" {
		t.Errorf("Expected eth0, got %s", cfg.InterfaceName)
	}
	if cfg.IPAddress != "10.0.2.15" {
		t.Errorf("Expected 10.0.2.15, got %s", cfg.IPAddress)
	}
	if cfg.NetmaskCIDR != 24 {
		t.Errorf("Expected CIDR 24, got %d", cfg.NetmaskCIDR)
	}
	if cfg.GatewayIP != "10.0.2.2" {
		t.Errorf("Expected gateway 10.0.2.2, got %s", cfg.GatewayIP)
	}
	if len(cfg.DNSServers) == 0 || cfg.DNSServers[0] != "10.0.2.3" {
		t.Errorf("Expected DNS 10.0.2.3, got %v", cfg.DNSServers)
	}
}

func TestWriteResolvConf(t *testing.T) {
	tmpDir := t.TempDir()
	resolvPath := filepath.Join(tmpDir, "etc", "resolv.conf")

	dnsList := []string{"10.0.2.3", "1.1.1.1", "8.8.8.8"}
	err := WriteResolvConf(dnsList, resolvPath)
	if err != nil {
		t.Fatalf("WriteResolvConf failed: %v", err)
	}

	data, err := os.ReadFile(resolvPath)
	if err != nil {
		t.Fatalf("Failed to read resolv.conf: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "nameserver 10.0.2.3") {
		t.Errorf("resolv.conf missing 10.0.2.3: %s", content)
	}
	if !strings.Contains(content, "nameserver 1.1.1.1") {
		t.Errorf("resolv.conf missing 1.1.1.1: %s", content)
	}
	if !strings.Contains(content, "nameserver 8.8.8.8") {
		t.Errorf("resolv.conf missing 8.8.8.8: %s", content)
	}
}

func TestEncodeRtAttrAlignment(t *testing.T) {
	data := []byte{10, 0, 2, 15}
	attr := encodeRtAttr(1, data)

	// Length should be unix.SizeofRtAttr (4) + len(data) (4) = 8 bytes
	if len(attr) != 8 {
		t.Errorf("Expected attr length 8, got %d", len(attr))
	}
}
