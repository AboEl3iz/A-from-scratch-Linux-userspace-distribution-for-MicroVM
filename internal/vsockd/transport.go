package vsockd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	DefaultVSockPort uint32 = 1024
	DefaultGuestCID  uint32 = 3
	DefaultHostCID   uint32 = 2
)

// VSockAddr implements net.Addr for AF_VSOCK addresses.
type VSockAddr struct {
	CID  uint32
	Port uint32
}

func (a *VSockAddr) Network() string { return "vsock" }
func (a *VSockAddr) String() string  { return fmt.Sprintf("%d:%d", a.CID, a.Port) }

// VSockConn implements net.Conn over an AF_VSOCK file descriptor.
type VSockConn struct {
	fd    int
	laddr VSockAddr
	raddr VSockAddr
}

func (c *VSockConn) Read(b []byte) (n int, err error) {
	n, err = unix.Read(c.fd, b)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *VSockConn) Write(b []byte) (n int, err error) {
	n, err = unix.Write(c.fd, b)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *VSockConn) Close() error {
	if c.fd >= 0 {
		err := unix.Close(c.fd)
		c.fd = -1
		return err
	}
	return nil
}

func (c *VSockConn) LocalAddr() net.Addr  { return &c.laddr }
func (c *VSockConn) RemoteAddr() net.Addr { return &c.raddr }

func (c *VSockConn) SetDeadline(t time.Time) error      { return nil }
func (c *VSockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *VSockConn) SetWriteDeadline(t time.Time) error { return nil }

// VSockListener implements net.Listener over an AF_VSOCK listening socket.
type VSockListener struct {
	fd   int
	addr VSockAddr
}

func (l *VSockListener) Accept() (net.Conn, error) {
	nfd, sa, err := unix.Accept(l.fd)
	if err != nil {
		return nil, err
	}
	unix.CloseOnExec(nfd)

	raddr := VSockAddr{CID: unix.VMADDR_CID_ANY, Port: 0}
	if vmSa, ok := sa.(*unix.SockaddrVM); ok {
		raddr.CID = vmSa.CID
		raddr.Port = vmSa.Port
	}

	return &VSockConn{
		fd:    nfd,
		laddr: l.addr,
		raddr: raddr,
	}, nil
}

func (l *VSockListener) Close() error {
	if l.fd >= 0 {
		err := unix.Close(l.fd)
		l.fd = -1
		return err
	}
	return nil
}

func (l *VSockListener) Addr() net.Addr { return &l.addr }

// ListenVSock creates a net.Listener bound to Linux AF_VSOCK stream socket on specified port.
func ListenVSock(port uint32) (net.Listener, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("failed creating AF_VSOCK socket: %w", err)
	}
	unix.CloseOnExec(fd)

	sa := &unix.SockaddrVM{
		CID:  unix.VMADDR_CID_ANY,
		Port: port,
	}

	if err := unix.Bind(fd, sa); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed binding AF_VSOCK socket to port %d: %w", port, err)
	}

	if err := unix.Listen(fd, 128); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed listening on AF_VSOCK socket port %d: %w", port, err)
	}

	return &VSockListener{
		fd:   fd,
		addr: VSockAddr{CID: unix.VMADDR_CID_ANY, Port: port},
	}, nil
}

// DialVSock connects to a remote AF_VSOCK endpoint (target CID and Port).
func DialVSock(cid uint32, port uint32) (net.Conn, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("failed creating AF_VSOCK socket: %w", err)
	}
	unix.CloseOnExec(fd)

	sa := &unix.SockaddrVM{
		CID:  cid,
		Port: port,
	}

	if err := unix.Connect(fd, sa); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed connecting AF_VSOCK socket to CID %d port %d: %w", cid, port, err)
	}

	return &VSockConn{
		fd:    fd,
		laddr: VSockAddr{CID: unix.VMADDR_CID_ANY, Port: 0},
		raddr: VSockAddr{CID: cid, Port: port},
	}, nil
}

// ListenUnix creates a Unix Domain Socket net.Listener, cleaning up stale socket files if present.
func ListenUnix(socketPath string) (net.Listener, error) {
	_ = os.Remove(socketPath)
	return net.Listen("unix", socketPath)
}

// DialUnix connects to a Unix Domain Socket endpoint.
func DialUnix(socketPath string) (net.Conn, error) {
	return net.Dial("unix", socketPath)
}

// Dial parses a transport address string ("vsock://3:1024", "unix:///tmp/v.sock", or "tcp://127.0.0.1:1024") and dials the remote server.
func Dial(address string) (net.Conn, error) {
	addr := strings.TrimSpace(address)
	if strings.HasPrefix(addr, "vsock://") {
		target := strings.TrimPrefix(addr, "vsock://")
		parts := strings.Split(target, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid vsock address format %q (expected vsock://CID:PORT)", addr)
		}
		cid, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid VSock CID in address %q: %w", addr, err)
		}
		port, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid VSock Port in address %q: %w", addr, err)
		}
		return DialVSock(uint32(cid), uint32(port))
	} else if strings.HasPrefix(addr, "unix://") {
		path := strings.TrimPrefix(addr, "unix://")
		return DialUnix(path)
	} else if strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".") {
		return DialUnix(addr)
	} else if strings.HasPrefix(addr, "tcp://") {
		target := strings.TrimPrefix(addr, "tcp://")
		return net.Dial("tcp", target)
	}

	return net.Dial("tcp", addr)
}

// IsVSockSupported returns true if /dev/vhost-vsock or AF_VSOCK is accessible on the host/guest kernel.
func IsVSockSupported() bool {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return false
	}
	_ = unix.Close(fd)
	return true
}

// EnsureFDIsNotInherited prevents socket leaking across exec boundaries
func EnsureFDIsNotInherited(fd int) {
	unix.CloseOnExec(fd)
}
