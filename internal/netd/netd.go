package netd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// NetworkConfig defines the IPv4 network parameters for a guest network interface.
type NetworkConfig struct {
	InterfaceName string
	IPAddress     string
	NetmaskCIDR   int
	GatewayIP     string
	DNSServers    []string
}

// DefaultConfig returns standard QEMU user-net defaults (10.0.2.15/24, GW 10.0.2.2, DNS 10.0.2.3).
func DefaultConfig(ifaceName string) NetworkConfig {
	if ifaceName == "" {
		ifaceName = "eth0"
	}
	return NetworkConfig{
		InterfaceName: ifaceName,
		IPAddress:     "10.0.2.15",
		NetmaskCIDR:   24,
		GatewayIP:     "10.0.2.2",
		DNSServers:    []string{"10.0.2.3", "1.1.1.1"},
	}
}

// AutoConfigure brings up the specified network interface, sets IPv4 address,
// configures default gateway route, and writes /etc/resolv.conf using pure Netlink syscalls.
func AutoConfigure(cfg NetworkConfig) error {
	if cfg.InterfaceName == "" {
		cfg.InterfaceName = "eth0"
	}

	fmt.Printf("[karim-netd] Configuring network interface %s...\n", cfg.InterfaceName)

	// 1. Bring link UP
	if err := SetLinkUp(cfg.InterfaceName); err != nil {
		return fmt.Errorf("failed to set link UP for %s: %w", cfg.InterfaceName, err)
	}
	fmt.Printf("[karim-netd] Interface %s is UP\n", cfg.InterfaceName)

	// 2. Set IPv4 Address
	if cfg.IPAddress != "" {
		if cfg.NetmaskCIDR <= 0 || cfg.NetmaskCIDR > 32 {
			cfg.NetmaskCIDR = 24
		}
		if err := SetInterfaceIP(cfg.InterfaceName, cfg.IPAddress, cfg.NetmaskCIDR); err != nil {
			return fmt.Errorf("failed to set IP %s/%d on %s: %w", cfg.IPAddress, cfg.NetmaskCIDR, cfg.InterfaceName, err)
		}
		fmt.Printf("[karim-netd] Set IP address %s/%d on %s\n", cfg.IPAddress, cfg.NetmaskCIDR, cfg.InterfaceName)
	}

	// 3. Set Default Gateway Route
	if cfg.GatewayIP != "" {
		if err := SetDefaultGateway(cfg.GatewayIP, cfg.InterfaceName); err != nil {
			// Log route error but don't crash if gateway already exists
			fmt.Printf("[karim-netd] Warning: failed to set default gateway %s: %v\n", cfg.GatewayIP, err)
		} else {
			fmt.Printf("[karim-netd] Configured default gateway %s via %s\n", cfg.GatewayIP, cfg.InterfaceName)
		}
	}

	// 4. Write /etc/resolv.conf
	if len(cfg.DNSServers) > 0 {
		if err := WriteResolvConf(cfg.DNSServers, "/etc/resolv.conf"); err != nil {
			fmt.Printf("[karim-netd] Warning: failed to write /etc/resolv.conf: %v\n", err)
		} else {
			fmt.Printf("[karim-netd] Wrote DNS configuration to /etc/resolv.conf (%s)\n", strings.Join(cfg.DNSServers, ", "))
		}
	}

	return nil
}

// SetLinkUp brings a network interface up by sending an RTM_NEWLINK netlink message.
func SetLinkUp(ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	file, err := openNetlinkSocketNonBlock()
	if err != nil {
		return err
	}
	defer file.Close()

	req := struct {
		Header unix.NlMsghdr
		Info   unix.IfInfomsg
	}{
		Header: unix.NlMsghdr{
			Len:   uint32(unix.SizeofNlMsghdr + unix.SizeofIfInfomsg),
			Type:  unix.RTM_NEWLINK,
			Flags: unix.NLM_F_REQUEST | unix.NLM_F_ACK,
			Seq:   1,
		},
		Info: unix.IfInfomsg{
			Family: unix.AF_UNSPEC,
			Index:  int32(iface.Index),
			Flags:  unix.IFF_UP,
			Change: unix.IFF_UP,
		},
	}

	buf := (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:]
	if err := unix.Sendto(int(file.Fd()), buf, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fmt.Errorf("netlink sendto failed: %w", err)
	}

	if err := readNetlinkAckPersistent(file, 1, 2*time.Second); err != nil {
		return err
	}

	return VerifyInterfaceUp(ifaceName)
}

// SetInterfaceIP assigns an IPv4 address and subnet prefix to the interface using RTM_NEWADDR.
func SetInterfaceIP(ifaceName, ipStr string, cidr int) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return fmt.Errorf("invalid IPv4 address: %s", ipStr)
	}

	file, err := openNetlinkSocketNonBlock()
	if err != nil {
		return err
	}
	defer file.Close()

	var buf bytes.Buffer

	// Netlink header
	nlhdr := unix.NlMsghdr{
		Type:  unix.RTM_NEWADDR,
		Flags: unix.NLM_F_REQUEST | unix.NLM_F_ACK | unix.NLM_F_CREATE | unix.NLM_F_REPLACE,
		Seq:   2,
	}

	msg := unix.IfAddrmsg{
		Family:    unix.AF_INET,
		Prefixlen: uint8(cidr),
		Flags:     0,
		Scope:     unix.RT_SCOPE_UNIVERSE,
		Index:     uint32(iface.Index),
	}

	// Build attributes: IFA_LOCAL and IFA_ADDRESS
	attrLocal := encodeRtAttr(unix.IFA_LOCAL, ip)
	attrAddress := encodeRtAttr(unix.IFA_ADDRESS, ip)

	payloadLen := unix.SizeofIfAddrmsg + len(attrLocal) + len(attrAddress)
	nlhdr.Len = uint32(unix.SizeofNlMsghdr + payloadLen)

	binary.Write(&buf, binary.LittleEndian, nlhdr)
	binary.Write(&buf, binary.LittleEndian, msg)
	buf.Write(attrLocal)
	buf.Write(attrAddress)

	if err := unix.Sendto(int(file.Fd()), buf.Bytes(), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fmt.Errorf("sendto RTM_NEWADDR failed: %w", err)
	}

	return readNetlinkAckPersistent(file, 2, 2*time.Second)
}

// SetDefaultGateway adds a default IPv4 route (0.0.0.0/0) pointing to gatewayIP on ifaceName.
func SetDefaultGateway(gatewayIP string, ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	gw := net.ParseIP(gatewayIP).To4()
	if gw == nil {
		return fmt.Errorf("invalid gateway IP: %s", gatewayIP)
	}

	file, err := openNetlinkSocketNonBlock()
	if err != nil {
		return err
	}
	defer file.Close()

	var buf bytes.Buffer

	nlhdr := unix.NlMsghdr{
		Type:  unix.RTM_NEWROUTE,
		Flags: unix.NLM_F_REQUEST | unix.NLM_F_ACK | unix.NLM_F_CREATE | unix.NLM_F_REPLACE,
		Seq:   3,
	}

	rtmsg := unix.RtMsg{
		Family:   unix.AF_INET,
		Dst_len:  0, // 0.0.0.0/0 (Default Gateway)
		Src_len:  0,
		Tos:      0,
		Table:    unix.RT_TABLE_MAIN,
		Protocol: unix.RTPROT_BOOT,
		Scope:    unix.RT_SCOPE_UNIVERSE,
		Type:     unix.RTN_UNICAST,
	}

	attrGw := encodeRtAttr(unix.RTA_GATEWAY, gw)
	ifindexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ifindexBytes, uint32(iface.Index))
	attrOif := encodeRtAttr(unix.RTA_OIF, ifindexBytes)

	payloadLen := unix.SizeofRtMsg + len(attrGw) + len(attrOif)
	nlhdr.Len = uint32(unix.SizeofNlMsghdr + payloadLen)

	binary.Write(&buf, binary.LittleEndian, nlhdr)
	binary.Write(&buf, binary.LittleEndian, rtmsg)
	buf.Write(attrGw)
	buf.Write(attrOif)

	if err := unix.Sendto(int(file.Fd()), buf.Bytes(), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fmt.Errorf("sendto RTM_NEWROUTE failed: %w", err)
	}

	return readNetlinkAckPersistent(file, 3, 2*time.Second)
}

// WriteResolvConf outputs nameserver directives into path (default /etc/resolv.conf).
func WriteResolvConf(dnsServers []string, path string) error {
	if path == "" {
		path = "/etc/resolv.conf"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory for resolv.conf: %w", err)
	}

	var content strings.Builder
	content.WriteString("# Generated by karim-netd\n")
	for _, dns := range dnsServers {
		dns = strings.TrimSpace(dns)
		if dns != "" {
			content.WriteString(fmt.Sprintf("nameserver %s\n", dns))
		}
	}

	return os.WriteFile(path, []byte(content.String()), 0644)
}

// Helper functions for non-blocking netlink socket operations integrated with Go netpoller

func openNetlinkSocketNonBlock() (*os.File, error) {
	fd, err := unix.Socket(
		unix.AF_NETLINK,
		unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC,
		unix.NETLINK_ROUTE,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create non-blocking netlink socket: %w", err)
	}

	sa := &unix.SockaddrNetlink{Family: unix.AF_NETLINK}
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to bind netlink socket: %w", err)
	}

	file := os.NewFile(uintptr(fd), "netlink-socket")
	if file == nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to wrap netlink socket in os.File")
	}

	return file, nil
}

func encodeRtAttr(attrType uint16, data []byte) []byte {
	attrLen := unix.SizeofRtAttr + len(data)
	alignedLen := (attrLen + 3) & ^3

	buf := make([]byte, alignedLen)
	binary.LittleEndian.PutUint16(buf[0:2], uint16(attrLen))
	binary.LittleEndian.PutUint16(buf[2:4], attrType)
	copy(buf[4:], data)

	return buf
}

func readNetlinkAckPersistent(file *os.File, expectedSeq uint32, timeout time.Duration) error {
	if err := file.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("failed to set netlink read deadline: %w", err)
	}

	rawConn, err := file.SyscallConn()
	if err != nil {
		return fmt.Errorf("failed to get syscall conn: %w", err)
	}

	rb := make([]byte, 8192)

	for {
		var n int
		var recvErr error

		err := rawConn.Read(func(fd uintptr) bool {
			n, _, recvErr = unix.Recvfrom(int(fd), rb, 0)
			if recvErr == unix.EAGAIN || recvErr == unix.EWOULDBLOCK {
				return false
			}
			return true
		})

		if err != nil {
			return fmt.Errorf("netlink read timed out or failed: %w", err)
		}
		if recvErr != nil {
			return fmt.Errorf("recvfrom netlink ack failed: %w", recvErr)
		}

		msgs, err := syscall.ParseNetlinkMessage(rb[:n])
		if err != nil {
			return fmt.Errorf("parse netlink msg failed: %w", err)
		}

		for _, msg := range msgs {
			if msg.Header.Seq != expectedSeq {
				continue
			}

			if msg.Header.Type == unix.NLMSG_ERROR {
				if len(msg.Data) >= 4 {
					errno := int32(binary.LittleEndian.Uint32(msg.Data[:4]))
					if errno != 0 {
						return fmt.Errorf("netlink error: %w", unix.Errno(-errno))
					}
				}
				return nil
			}
		}
	}
}

// VerifyInterfaceUp checks /sys/class/net/<iface>/operstate to independently confirm link state.
func VerifyInterfaceUp(ifaceName string) error {
	path := fmt.Sprintf("/sys/class/net/%s/operstate", ifaceName)
	data, err := os.ReadFile(path)
	if err != nil {
		// If sysfs entry doesn't exist yet or is inaccessible, fallback gracefully
		return nil
	}

	state := strings.TrimSpace(string(data))
	if state != "up" && state != "unknown" && state != "lowerup" {
		return fmt.Errorf("interface %s operstate is %q (expected 'up', 'unknown', or 'lowerup')", ifaceName, state)
	}

	return nil
}

