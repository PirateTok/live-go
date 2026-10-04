package connection

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/PirateTok/live-go/events"
	"github.com/PirateTok/live-go/internal/fakes"
	pb "github.com/PirateTok/live-go/proto"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"google.golang.org/protobuf/proto"
)

type seen struct {
	header http.Header
	query  url.Values
	frames []*pb.WebcastPushFrame // client → server, in order
}

// fakeWebcast upgrades one connection, sends a needs_ack msg frame carrying a
// chat message, waits for the ack, then closes.
func fakeWebcast(t *testing.T, done chan<- seen) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := seen{header: r.Header.Clone(), query: r.URL.Query()}
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		chat, _ := proto.Marshal(&pb.WebcastChatMessage{Content: "hello from fake"})
		resp, _ := proto.Marshal(&pb.WebcastResponse{
			Messages:    []*pb.WebcastResponse_Message{{Method: "WebcastChatMessage", Payload: chat}},
			InternalExt: []byte("ext-blob"),
			NeedsAck:    true,
		})
		push, _ := proto.Marshal(&pb.WebcastPushFrame{LogId: 4242, PayloadType: "msg", Payload: resp})

		sent := false
		for {
			data, err := wsutil.ReadClientBinary(conn)
			if err != nil {
				t.Errorf("read client frame: %v", err)
				done <- s
				return
			}
			f := &pb.WebcastPushFrame{}
			if err := proto.Unmarshal(data, f); err != nil {
				t.Errorf("client frame: %v", err)
			}
			s.frames = append(s.frames, f)
			if f.PayloadType == "im_enter_room" && !sent {
				wsutil.WriteServerBinary(conn, push)
				sent = true
			}
			if f.PayloadType == "ack" {
				done <- s
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func payloadTypes(frames []*pb.WebcastPushFrame) []string {
	var out []string
	for _, f := range frames {
		out = append(out, f.PayloadType)
	}
	return out
}

// F2 + F7 + F8: heartbeat, enter_room, ack (log_id + internal_ext), and the
// UA / cookies / language / region / compress / heartbeat_duration on the wire.
func TestRunWebSocketAgainstFakeWebcast(t *testing.T) {
	done := make(chan seen, 1)
	srv := fakeWebcast(t, done)
	wsURL := strings.Replace(
		BuildWSSURL("HOST", "7", "UTC", "ro", "RO", false, 2*time.Second),
		"wss://HOST", "ws://"+srv.Listener.Addr().String(), 1)

	evCh := make(chan events.Event, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go RunWebSocket(ctx, wsURL, "ttwid=abc; sessionid=s1", "UA-test/1.0", "7", 2*time.Second, 5*time.Second, "ro-RO,ro;q=0.9", "", evCh)

	var s seen
	select {
	case s = <-done:
	case <-ctx.Done():
		t.Fatal("fake webcast never saw an ack")
	}

	types := payloadTypes(s.frames)
	if len(types) < 3 || types[0] != "hb" || types[1] != "im_enter_room" || types[len(types)-1] != "ack" {
		t.Fatalf("client frames = %v, want hb, im_enter_room, ..., ack", types)
	}
	ack := s.frames[len(s.frames)-1]
	if ack.LogId != 4242 || !bytes.Equal(ack.Payload, []byte("ext-blob")) {
		t.Fatalf("ack log_id=%d payload=%q", ack.LogId, ack.Payload)
	}

	wantHeaders := map[string]string{
		"User-Agent":      "UA-test/1.0",
		"Cookie":          "ttwid=abc; sessionid=s1",
		"Accept-Language": "ro-RO,ro;q=0.9",
		"Origin":          "https://www.tiktok.com",
	}
	for k, v := range wantHeaders {
		if got := s.header.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
	wantQuery := map[string]string{
		"room_id":            "7",
		"webcast_language":   "ro",
		"app_language":       "ro",
		"browser_language":   "ro-RO",
		"compress":           "",
		"heartbeat_duration": "2000",
	}
	for k, v := range wantQuery {
		if !s.query.Has(k) || s.query.Get(k) != v {
			t.Errorf("query %s = %q (present=%v), want %q", k, s.query.Get(k), s.query.Has(k), v)
		}
	}

	select {
	case ev := <-evCh:
		chat, ok := ev.Data.(*pb.WebcastChatMessage)
		if ev.Type != events.EventChat || !ok || chat.Content != "hello from fake" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no chat event decoded")
	}
}

const wssTarget = "wss://webcast-ws.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/?room_id=7"

func runThrough(t *testing.T, proxy string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RunWebSocket(ctx, wssTarget, "ttwid=abc", "ua", "7", time.Second, time.Second, "en-US,en;q=0.9", proxy, make(chan events.Event, 1))
}

// F6: WSS must tunnel through an HTTP CONNECT proxy with credentials.
func TestWSSThroughConnectProxy(t *testing.T) {
	p := fakes.ConnectProxy(t)
	if err := runThrough(t, "http://user:pw@"+p.Addr); err == nil {
		t.Fatal("expected error: fake proxy refuses the tunnel")
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pw"))
	hits := p.Hits()
	if len(hits) != 1 || hits[0].Target != "webcast-ws.tiktok.com:443" || hits[0].Auth != want {
		t.Fatalf("proxy saw %+v", hits)
	}
}

// F6: WSS must tunnel through a SOCKS5 proxy with credentials.
func TestWSSThroughSocks5Proxy(t *testing.T) {
	p := fakes.Socks5Proxy(t)
	if err := runThrough(t, "socks5://user:pw@"+p.Addr); err == nil {
		t.Fatal("expected error: fake proxy refuses the tunnel")
	}
	hits := p.Hits()
	if len(hits) != 1 || hits[0].Target != "webcast-ws.tiktok.com:443" || hits[0].Auth != "user:pw" {
		t.Fatalf("socks5 proxy saw %+v", hits)
	}
}

func TestUnsupportedProxySchemeFails(t *testing.T) {
	if err := runThrough(t, "ftp://127.0.0.1:1"); err == nil || !strings.Contains(err.Error(), "unsupported proxy scheme") {
		t.Fatalf("err = %v", err)
	}
}

// F6 env fallback. net/http caches proxy env vars per process, so the check
// runs in a child process with HTTPS_PROXY set from the start.
func TestWSSUsesEnvProxyWhenNoneConfigured(t *testing.T) {
	if addr := os.Getenv("PIRATETOK_ENV_PROXY_CHILD"); addr != "" {
		if err := runThrough(t, ""); err == nil {
			t.Fatal("expected error: fake proxy refuses the tunnel")
		}
		return
	}
	p := fakes.ConnectProxy(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestWSSUsesEnvProxyWhenNoneConfigured$")
	cmd.Env = append(os.Environ(), "PIRATETOK_ENV_PROXY_CHILD="+p.Addr, "HTTPS_PROXY=http://"+p.Addr, "NO_PROXY=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	hits := p.Hits()
	if len(hits) != 1 || hits[0].Target != "webcast-ws.tiktok.com:443" {
		t.Fatalf("env proxy saw %+v", hits)
	}
}
