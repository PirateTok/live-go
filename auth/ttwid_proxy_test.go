package auth

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/PirateTok/live-go/internal/fakes"
)

// F6: the ttwid GET must go through the configured proxy, with credentials.
func TestFetchTTWIDThroughConnectProxy(t *testing.T) {
	p := fakes.ConnectProxy(t)
	if _, err := FetchTTWID(2*time.Second, "test-ua", "http://user:pw@"+p.Addr); err == nil {
		t.Fatal("expected error: fake proxy refuses the tunnel")
	}
	hits := p.Hits()
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pw"))
	if len(hits) != 1 || hits[0].Target != "www.tiktok.com:443" || hits[0].Auth != want {
		t.Fatalf("proxy saw %+v, want CONNECT www.tiktok.com:443 with %q", hits, want)
	}
}

func TestFetchTTWIDThroughSocks5Proxy(t *testing.T) {
	p := fakes.Socks5Proxy(t)
	if _, err := FetchTTWID(2*time.Second, "test-ua", "socks5://user:pw@"+p.Addr); err == nil {
		t.Fatal("expected error: fake proxy refuses the tunnel")
	}
	hits := p.Hits()
	if len(hits) != 1 || hits[0].Target != "www.tiktok.com:443" || hits[0].Auth != "user:pw" {
		t.Fatalf("socks5 proxy saw %+v", hits)
	}
}
