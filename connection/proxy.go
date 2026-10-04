package connection

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

// resolveProxy returns the explicit proxy, else the environment proxy for the
// https:// equivalent of the WSS target (HTTPS_PROXY / ALL_PROXY / NO_PROXY).
// nil means dial directly.
func resolveProxy(explicit string, wssURL string) (*url.URL, error) {
	if explicit != "" {
		u, err := url.Parse(explicit)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		return u, nil
	}
	target, err := url.Parse(wssURL)
	if err != nil {
		return nil, fmt.Errorf("invalid wss URL: %w", err)
	}
	scheme := "https"
	if target.Scheme == "ws" {
		scheme = "http"
	}
	req := &http.Request{URL: &url.URL{Scheme: scheme, Host: target.Host}}
	return http.ProxyFromEnvironment(req)
}

type netDialFunc = func(ctx context.Context, network, addr string) (net.Conn, error)

// proxyNetDial returns a NetDial that tunnels through the proxy: HTTP CONNECT
// for http/https, the SOCKS5 handshake (with user/pass auth) for socks5/socks5h.
func proxyNetDial(proxyURL *url.URL) (netDialFunc, error) {
	switch proxyURL.Scheme {
	case "http", "https", "":
		return func(ctx context.Context, _, addr string) (net.Conn, error) {
			return dialThrough(ctx, proxyURL, "80", func(c net.Conn) error { return httpConnect(c, proxyURL.User, addr) })
		}, nil
	case "socks5", "socks5h":
		return func(ctx context.Context, _, addr string) (net.Conn, error) {
			return dialThrough(ctx, proxyURL, "1080", func(c net.Conn) error { return socks5Connect(c, proxyURL.User, addr) })
		}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (http, https, socks5, socks5h)", proxyURL.Scheme)
	}
}

func dialThrough(ctx context.Context, proxyURL *url.URL, defaultPort string, handshake func(net.Conn) error) (net.Conn, error) {
	host := proxyURL.Host
	if proxyURL.Port() == "" {
		if proxyURL.Scheme == "https" {
			defaultPort = "443"
		}
		host = net.JoinHostPort(proxyURL.Hostname(), defaultPort)
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("proxy dial %s: %w", host, err)
	}
	if err := handshake(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func httpConnect(conn net.Conn, user *url.Userinfo, addr string) error {
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if user != nil {
		password, _ := user.Password()
		creds := base64.StdEncoding.EncodeToString([]byte(user.Username() + ":" + password))
		req += "Proxy-Authorization: Basic " + creds + "\r\n"
	}
	if _, err := io.WriteString(conn, req+"\r\n"); err != nil {
		return fmt.Errorf("proxy CONNECT write: %w", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return fmt.Errorf("proxy CONNECT response: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy CONNECT failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// socks5Connect runs the RFC 1928 client handshake, with RFC 1929 user/pass
// auth when the proxy URL has credentials. The target host is sent as a
// domain name so the proxy resolves it.
func socks5Connect(conn net.Conn, user *url.Userinfo, addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("socks5: bad target %q: %w", addr, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return fmt.Errorf("socks5: bad port %q: %w", portStr, err)
	}
	if len(host) > 255 {
		return fmt.Errorf("socks5: host too long")
	}

	method := byte(0x00)
	if user != nil {
		method = 0x02
	}
	if _, err := conn.Write([]byte{0x05, 0x01, method}); err != nil {
		return fmt.Errorf("socks5 greeting: %w", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("socks5 greeting reply: %w", err)
	}
	if reply[0] != 0x05 || reply[1] != method {
		return fmt.Errorf("socks5: proxy refused auth method %d (got %d)", method, reply[1])
	}

	if user != nil {
		password, _ := user.Password()
		name := user.Username()
		if len(name) > 255 || len(password) > 255 {
			return fmt.Errorf("socks5: credentials too long")
		}
		auth := append([]byte{0x01, byte(len(name))}, name...)
		auth = append(append(auth, byte(len(password))), password...)
		if _, err := conn.Write(auth); err != nil {
			return fmt.Errorf("socks5 auth: %w", err)
		}
		if _, err := io.ReadFull(conn, reply); err != nil {
			return fmt.Errorf("socks5 auth reply: %w", err)
		}
		if reply[1] != 0x00 {
			return fmt.Errorf("socks5: authentication failed")
		}
	}

	req := append([]byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}, host...)
	req = binary.BigEndian.AppendUint16(req, uint16(port))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5 connect: %w", err)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return fmt.Errorf("socks5 connect reply: %w", err)
	}
	if head[1] != 0x00 {
		return fmt.Errorf("socks5: connect to %s failed (reply %d)", addr, head[1])
	}
	var skip int
	switch head[3] {
	case 0x01:
		skip = 4
	case 0x04:
		skip = 16
	case 0x03:
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return fmt.Errorf("socks5 bound addr: %w", err)
		}
		skip = int(l[0])
	default:
		return fmt.Errorf("socks5: bad address type %d", head[3])
	}
	if _, err := io.ReadFull(conn, make([]byte, skip+2)); err != nil {
		return fmt.Errorf("socks5 bound addr: %w", err)
	}
	return nil
}
