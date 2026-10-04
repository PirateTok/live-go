// Package fakes holds offline test doubles: proxies that record what the
// client sent and then refuse the tunnel, so no traffic leaves the machine.
package fakes

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
)

// Hit is one tunnel request seen by a fake proxy.
type Hit struct {
	Target string // host:port the client asked for
	Auth   string // Proxy-Authorization header (CONNECT) or "user:pass" (SOCKS5)
}

// Proxy is a fake proxy listening on 127.0.0.1.
type Proxy struct {
	Addr string
	mu   sync.Mutex
	hits []Hit
}

// Hits returns the tunnel requests seen so far.
func (p *Proxy) Hits() []Hit {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Hit(nil), p.hits...)
}

func (p *Proxy) record(h Hit) {
	p.mu.Lock()
	p.hits = append(p.hits, h)
	p.mu.Unlock()
}

func listen(t *testing.T, serve func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				serve(c)
			}()
		}
	}()
	return ln.Addr().String()
}

// ConnectProxy records each CONNECT request line + Proxy-Authorization, then answers 502.
func ConnectProxy(t *testing.T) *Proxy {
	p := &Proxy{}
	p.Addr = listen(t, func(c net.Conn) {
		req, err := http.ReadRequest(bufio.NewReader(c))
		if err != nil {
			return
		}
		target := req.Host
		if req.Method != http.MethodConnect {
			target = req.Method + " " + req.URL.String()
		}
		p.record(Hit{Target: target, Auth: req.Header.Get("Proxy-Authorization")})
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
	})
	return p
}

// Socks5Proxy speaks RFC 1928/1929 far enough to record the target and
// credentials, then replies "connection refused".
func Socks5Proxy(t *testing.T) *Proxy {
	p := &Proxy{}
	p.Addr = listen(t, func(c net.Conn) {
		hit, err := socks5Handshake(c)
		if err != nil {
			return
		}
		p.record(hit)
		c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	})
	return p
}

func socks5Handshake(c net.Conn) (Hit, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(c, head); err != nil {
		return Hit{}, err
	}
	methods := make([]byte, head[1])
	if _, err := io.ReadFull(c, methods); err != nil {
		return Hit{}, err
	}
	var hit Hit
	if contains(methods, 0x02) {
		c.Write([]byte{0x05, 0x02})
		ver := make([]byte, 2)
		if _, err := io.ReadFull(c, ver); err != nil {
			return Hit{}, err
		}
		user := make([]byte, ver[1])
		io.ReadFull(c, user)
		plen := make([]byte, 1)
		io.ReadFull(c, plen)
		pass := make([]byte, plen[0])
		io.ReadFull(c, pass)
		hit.Auth = string(user) + ":" + string(pass)
		c.Write([]byte{0x01, 0x00})
	} else {
		c.Write([]byte{0x05, 0x00})
	}
	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil {
		return Hit{}, err
	}
	var host string
	switch req[3] {
	case 0x03:
		l := make([]byte, 1)
		io.ReadFull(c, l)
		name := make([]byte, l[0])
		io.ReadFull(c, name)
		host = string(name)
	case 0x01:
		ip := make([]byte, 4)
		io.ReadFull(c, ip)
		host = net.IP(ip).String()
	default:
		return Hit{}, fmt.Errorf("atyp %d", req[3])
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(c, port); err != nil {
		return Hit{}, err
	}
	hit.Target = net.JoinHostPort(host, fmt.Sprint(binary.BigEndian.Uint16(port)))
	return hit, nil
}

func contains(b []byte, v byte) bool {
	for _, x := range b {
		if x == v {
			return true
		}
	}
	return false
}
