// Package socks5 provides a minimal in-process SOCKS5 server used by tests
// to verify that requests are really routed through a proxy.
//
// The server accepts the SOCKS5 greeting, always permits the no-authentication
// method, handles only the CONNECT command and tunnels the resulting
// connection. It records every target the clients asked to tunnel to, so a
// test can assert that a request went through the proxy.
package socks5

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Server is a single-listener SOCKS5 server that accepts CONNECT requests and
// tunnels them to the requested target.
type Server struct {
	lis     net.Listener
	mu      sync.Mutex
	tunnels []string
}

// New starts a server on 127.0.0.1:port (port 0 → a free port).
func New(port int) (*Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, err
	}
	s := &Server{lis: lis}
	go s.loop()
	return s, nil
}

// Addr returns the actual (host:port) the server is listening on.
func (s *Server) Addr() string {
	return s.lis.Addr().String()
}

// Close stops accepting new connections.
func (s *Server) Close() {
	s.lis.Close()
}

// Tunnels returns a snapshot of the targets clients asked to tunnel to.
func (s *Server) Tunnels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.tunnels))
	copy(out, s.tunnels)
	return out
}

func (s *Server) loop() {
	for {
		c, err := s.lis.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

// handle processes a single SOCKS5 connection: handshake, CONNECT, pump.
func (s *Server) handle(c net.Conn) {
	c.SetReadDeadline(time.Now().Add(15 * time.Second))
	dst, err := s.handshake(c)
	if err != nil {
		c.Close()
		return
	}
	conn, dialErr := net.Dial("tcp", dst.String())
	s.writeReply(c, dst, dialErr)
	if dialErr != nil {
		c.Close()
		return
	}
	s.mu.Lock()
	s.tunnels = append(s.tunnels, dst.String())
	s.mu.Unlock()
	// Generous, fresh deadlines so a stuck peer can never hang the pump.
	c.SetReadDeadline(time.Now().Add(15 * time.Second))
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	go s.pump(c, conn)
}

// handshake reads the SOCKS5 greeting and CONNECT request; returns the target.
func (s *Server) handshake(c net.Conn) (net.Addr, error) {
	// 1. read the full SOCKS5 greeting: VER, NMETHODS, methods...
	buf := make([]byte, 2)
	if _, err := io.ReadFull(c, buf); err != nil {
		return nil, err
	}
	if buf[0] != 0x05 {
		return nil, fmt.Errorf("unsupported SOCKS version %d", buf[0])
	}
	methods := make([]byte, buf[1])
	if _, err := io.ReadFull(c, methods); err != nil {
		return nil, err
	}
	_ = methods

	// 2. reply: permit no-authentication.
	if _, err := c.Write([]byte{0x05, 0x00}); err != nil {
		return nil, err
	}

	// 3. read the CONNECT request: VER, CMD, RSV, ATYP.
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(c, hdr); err != nil {
		return nil, err
	}
	cmd, addrType := hdr[1], hdr[3]
	if cmd != 0x01 {
		return nil, fmt.Errorf("unsupported cmd %d", cmd)
	}
	switch addrType {
	case 0x01: // INET
		ip := make([]byte, 4)
		if _, err := io.ReadFull(c, ip); err != nil {
			return nil, err
		}
		p := make([]byte, 2)
		if _, err := io.ReadFull(c, p); err != nil {
			return nil, err
		}
		return &net.TCPAddr{IP: ip, Port: int(p[0])*256 + int(p[1])}, nil
	case 0x03: // FQDN
		nb := make([]byte, 1)
		if _, err := io.ReadFull(c, nb); err != nil {
			return nil, err
		}
		name := make([]byte, nb[0])
		if _, err := io.ReadFull(c, name); err != nil {
			return nil, err
		}
		p := make([]byte, 2)
		if _, err := io.ReadFull(c, p); err != nil {
			return nil, err
		}
		host, err := net.ResolveTCPAddr("tcp", string(name))
		if err != nil {
			return nil, err
		}
		return host, nil
	default:
		return nil, fmt.Errorf("unsupported addr type %d", addrType)
	}
}

// writeReply sends the SOCKS5 reply (VER, code, RSV, ATYP, ADDR, DSTPORT).
func (s *Server) writeReply(c net.Conn, dst net.Addr, err error) {
	code := byte(0x00)
	if err != nil {
		code = 0x03 // general SOCKS server failure
	}
	buf := []byte{0x05, code, 0x00}
	if tcp, ok := dst.(*net.TCPAddr); ok {
		if len(tcp.IP) == 4 {
			buf = append(buf, 0x01)
			buf = append(buf, tcp.IP...)
		} else {
			buf = append(buf, 0x04)
			buf = append(buf, tcp.IP...)
		}
		buf = append(buf, byte(tcp.Port>>8), byte(tcp.Port))
	}
	if _, werr := c.Write(buf); werr != nil {
		// Best effort: the tunnel reply is the client's last handshake byte;
		// any failure surfaces to the caller as a dial error.
	}
}

// pump copies bytes both ways between the client and the destination.
func (s *Server) pump(client, dest net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(dest, client)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, dest)
		done <- struct{}{}
	}()
	<-done
	<-done
	client.Close()
	dest.Close()
}
