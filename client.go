//go:generate go run discipline/scanner.go .

package golive

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/PirateTok/live-go/auth"
	"github.com/PirateTok/live-go/connection"
	"github.com/PirateTok/live-go/events"
	tthttp "github.com/PirateTok/live-go/http"
)

// Client connects to TikTok Live streams and emits events.
type Client struct {
	username          string
	cdnHost           string
	timeout           time.Duration
	heartbeatInterval time.Duration
	maxRetries        int
	staleTimeout      time.Duration
	userAgent         string
	cookies           string
	language          string
	region            string
	proxy             string
	compress          bool
}

// NewClient creates a new TikTok Live client for the given username.
func NewClient(username string) *Client {
	return &Client{
		username:          username,
		cdnHost:           "webcast-ws.tiktok.com",
		timeout:           10 * time.Second,
		heartbeatInterval: 10 * time.Second,
		maxRetries:        5,
		staleTimeout:      60 * time.Second,
		compress:          true,
	}
}

// HeartbeatInterval sets the interval between WSS heartbeat frames. Defaults
// to 10s. Also sent to TikTok as the heartbeat_duration URL param (ms).
func (c *Client) HeartbeatInterval(d time.Duration) *Client {
	c.heartbeatInterval = d
	return c
}

// CdnEU sets the CDN endpoint to EU.
func (c *Client) CdnEU() *Client {
	c.cdnHost = "webcast-ws.eu.tiktok.com"
	return c
}

// CdnUS sets the CDN endpoint to US.
func (c *Client) CdnUS() *Client {
	c.cdnHost = "webcast-ws.us.tiktok.com"
	return c
}

// Cdn sets a custom CDN host.
func (c *Client) Cdn(host string) *Client {
	c.cdnHost = host
	return c
}

// Timeout sets the HTTP timeout for API calls.
func (c *Client) Timeout(d time.Duration) *Client {
	c.timeout = d
	return c
}

// MaxRetries sets the max consecutive failed reconnection attempts. Defaults
// to 5. A session that stayed up 30s resets the count.
func (c *Client) MaxRetries(n int) *Client {
	c.maxRetries = n
	return c
}

// StaleTimeout sets the stale connection timeout. Defaults to 60s.
func (c *Client) StaleTimeout(d time.Duration) *Client {
	c.staleTimeout = d
	return c
}

// UserAgent sets a custom user agent. When empty, a random UA is picked from
// the built-in pool on each reconnect (recommended -- reduces DEVICE_BLOCKED risk).
func (c *Client) UserAgent(ua string) *Client {
	c.userAgent = ua
	return c
}

// Cookies sets extra session cookies appended alongside ttwid in the WSS
// Cookie header. Only needed to pass authenticated cookies (e.g. "sessionid=xxx; sid_tt=xxx").
// For room info on 18+ rooms, pass cookies directly to FetchRoomInfo() instead.
func (c *Client) Cookies(cookies string) *Client {
	c.cookies = cookies
	return c
}

// Language overrides the detected system language (e.g. "pt", "ro").
func (c *Client) Language(lang string) *Client {
	c.language = lang
	return c
}

// Region overrides the detected system region (e.g. "BR", "RO").
func (c *Client) Region(reg string) *Client {
	c.region = reg
	return c
}

// Proxy sets an HTTP/HTTPS/SOCKS5 proxy URL for all HTTP and WSS connections.
// When empty, falls back to HTTP_PROXY/HTTPS_PROXY environment variables.
func (c *Client) Proxy(url string) *Client {
	c.proxy = url
	return c
}

// Compress enables or disables gzip compression for the WSS connection.
// Defaults to true. When false, the server sends uncompressed protobuf frames.
func (c *Client) Compress(enabled bool) *Client {
	c.compress = enabled
	return c
}

// Connect resolves the room, then enters a reconnect loop.
// Events are sent to the returned channel. The channel is closed when done.
func (c *Client) Connect(ctx context.Context) (<-chan events.Event, error) {
	lang := c.language
	if lang == "" {
		lang = tthttp.SystemLanguage()
	}
	reg := c.region
	if reg == "" {
		reg = tthttp.SystemRegion()
	}
	acceptLang := fmt.Sprintf("%s-%s,%s;q=0.9", lang, reg, lang)

	room, err := tthttp.CheckOnline(c.username, c.timeout, lang, reg, c.proxy)
	if err != nil {
		return nil, err
	}

	tz := tthttp.SystemTimezone()
	eventCh := make(chan events.Event, 256)
	eventCh <- events.Event{Type: events.EventConnected, RoomID: room.RoomID}

	deps := loopDeps{
		fresh: c.freshSession,
		run: func(ctx context.Context, s *session) sessionExit {
			return c.runSession(ctx, s, room.RoomID, tz, lang, reg, acceptLang, eventCh)
		},
		sleep: sleepCtx,
	}
	go supervise(ctx, room.RoomID, c.maxRetries, deps, eventCh)

	return eventCh, nil
}

// session is the ttwid + UA pair reused across reconnects.
type session struct {
	ttwid string
	ua    string
}

// freshSession picks a UA and fetches a ttwid; nil means the fetch failed.
func (c *Client) freshSession() *session {
	ua := c.userAgent
	if ua == "" {
		ua = tthttp.RandomUA()
	}
	ttwid, err := auth.FetchTTWID(c.timeout, ua, c.proxy)
	if err != nil {
		log.Printf("ttwid acquisition failed: %v", err)
		return nil
	}
	return &session{ttwid: ttwid, ua: ua}
}

func (c *Client) runSession(ctx context.Context, s *session, roomID, tz, lang, reg, acceptLang string, eventCh chan events.Event) sessionExit {
	cookieHeader := fmt.Sprintf("ttwid=%s", s.ttwid)
	if c.cookies != "" {
		cookieHeader = fmt.Sprintf("ttwid=%s; %s", s.ttwid, c.cookies)
	}
	wssURL := connection.BuildWSSURL(c.cdnHost, roomID, tz, lang, reg, c.compress, c.heartbeatInterval)
	err := connection.RunWebSocket(ctx, wssURL, cookieHeader, s.ua, roomID, c.heartbeatInterval, c.staleTimeout, acceptLang, c.proxy, eventCh)
	if err == nil {
		return exitClosed
	}
	var dbErr *connection.DeviceBlockedError
	if errors.As(err, &dbErr) {
		log.Printf("DEVICE_BLOCKED -- rotating ttwid + UA")
		return exitDeviceBlocked
	}
	log.Printf("wss error: %v", err)
	return exitErrored
}

// CheckOnline checks if a user is currently live without connecting.
// Language and region auto-detected from system locale.
func CheckOnline(username string, timeout time.Duration) (*tthttp.RoomIDResult, error) {
	return tthttp.CheckOnline(username, timeout, "", "", "")
}

// FetchRoomInfo fetches optional room metadata. Pass cookies for 18+ rooms.
// Language and region auto-detected from system locale.
func FetchRoomInfo(roomID string, timeout time.Duration, cookies string) (*tthttp.RoomInfo, error) {
	return tthttp.FetchRoomInfo(roomID, timeout, cookies, "", "", "")
}

// FetchRoomAudience fetches the full viewer roster. Login-gated: session
// cookies are required for this call only (*tthttp.SessionRequiredError
// otherwise). Pass "" as anchorID to resolve it from room info.
// Language and region auto-detected from system locale.
func FetchRoomAudience(roomID string, anchorID string, timeout time.Duration, cookies string) (*tthttp.RoomAudience, error) {
	return tthttp.FetchRoomAudience(roomID, anchorID, timeout, cookies, "", "", "")
}
