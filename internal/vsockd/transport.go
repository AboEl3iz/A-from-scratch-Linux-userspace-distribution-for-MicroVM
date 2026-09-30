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

// VSockConn implements net.Conn over an AF_VSOCK file descriptor integrated with Go netpoller.
type VSockConn struct {
	conn  net.Conn
	laddr VSockAddr
	raddr VSockAddr
}

func (c *VSockConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *VSockConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *VSockConn) Close() error {
	return c.conn.Close()
}

func (c *VSockConn) LocalAddr() net.Addr  { return &c.laddr }
func (c *VSockConn) RemoteAddr() net.Addr { return &c.raddr }

func (c *VSockConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *VSockConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *VSockConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

// VSockListener implements net.Listener over an AF_VSOCK listening socket integrated with Go netpoller.
type VSockListener struct {
	listener net.Listener
	addr     VSockAddr
}

func (l *VSockListener) Accept() (net.Conn, error) {
	c, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	raddr := VSockAddr{CID: unix.VMADDR_CID_ANY, Port: 0}
	return &VSockConn{
		conn:  c,
		laddr: l.addr,
		raddr: raddr,
	}, nil
}

func (l *VSockListener) Close() error {
	return l.listener.Close()
}

func (l *VSockListener) Addr() net.Addr { return &l.addr }

// ListenVSock creates a net.Listener bound to Linux AF_VSOCK stream socket on specified port using Go netpoller.
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

	file := os.NewFile(uintptr(fd), "vsock-listener")
	defer file.Close()

	l, err := net.FileListener(file)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed registering AF_VSOCK listener with epoll netpoller: %w", err)
	}

	return &VSockListener{
		listener: l,
		addr:     VSockAddr{CID: unix.VMADDR_CID_ANY, Port: port},
	}, nil
}

// DialVSock connects to a remote AF_VSOCK endpoint (target CID and Port) using Go netpoller.
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

	file := os.NewFile(uintptr(fd), "vsock-conn")
	defer file.Close()

	c, err := net.FileConn(file)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed registering AF_VSOCK conn with epoll netpoller: %w", err)
	}

	return &VSockConn{
		conn:  c,
		laddr: VSockAddr{CID: unix.VMADDR_CID_ANY, Port: 0},
		raddr: VSockAddr{CID: cid, Port: port},
	}, nil
}

// ListenUnix creates a Unix Domain Socket net.Listener, enforcing strict 0600 permissions.
func ListenUnix(socketPath string) (net.Listener, error) {
	_ = os.Remove(socketPath)
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("failed setting 0600 permissions on socket %s: %w", socketPath, err)
	}
	return l, nil
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
