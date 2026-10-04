package http

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/PirateTok/live-go/internal/fakes"
)

// F6: API calls go through the configured proxy (CONNECT + SOCKS5), with credentials.
func TestCheckOnlineThroughProxies(t *testing.T) {
	connect := fakes.ConnectProxy(t)
	if _, err := CheckOnline("someone", 2*time.Second, "en", "US", "http://user:pw@"+connect.Addr); err == nil {
		t.Fatal("expected error: fake proxy refuses the tunnel")
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pw"))
	if h := connect.Hits(); len(h) != 1 || h[0].Target != "www.tiktok.com:443" || h[0].Auth != want {
		t.Fatalf("connect proxy saw %+v", h)
	}

	socks := fakes.Socks5Proxy(t)
	if _, err := FetchRoomInfo("7", 2*time.Second, "", "en", "US", "socks5://user:pw@"+socks.Addr); err == nil {
		t.Fatal("expected error: fake proxy refuses the tunnel")
	}
	if h := socks.Hits(); len(h) != 1 || h[0].Target != "webcast.tiktok.com:443" || h[0].Auth != "user:pw" {
		t.Fatalf("socks5 proxy saw %+v", h)
	}
}
