//go:generate go run discipline/scanner.go .

package golive

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/PirateTok/live-go/auth"
	"github.com/PirateTok/live-go/connection"
	"github.com/PirateTok/live-go/events"
	tthttp "github.com/PirateTok/live-go/http"
)

// Client connects to TikTok Live streams and emits events.
type Client struct {
	username     string
	cdnHost      string
	timeout      time.Duration
	maxRetries   int
	staleTimeout time.Duration
	userAgent    string
	cookies      string
	language     string
	region       string
	proxy        string
	compress     bool
}

// NewClient creates a new TikTok Live client for the given username.
func NewClient(username string) *Client {
	return &Client{
		username:     username,
		cdnHost:      "webcast-ws.tiktok.com",
		timeout:      10 * time.Second,
		maxRetries:   5,
		staleTimeout: 60 * time.Second,
		compress:     true,
	}
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

// MaxRetries sets the max reconnection attempts. Defaults to 5.
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

	go func() {
		defer close(eventCh)
		attempt := 0
		for {
			if ctx.Err() != nil {
				break
			}

			// Pick UA: user-configured or random from pool
			ua := c.userAgent
			if ua == "" {
				ua = tthttp.RandomUA()
			}

			ttwid, err := auth.FetchTTWID(c.timeout, ua, c.proxy)
			if err != nil {
				log.Printf("ttwid fetch failed: %v", err)
				break
			}

			// Build cookie header: ttwid + optional user cookies
			cookieHeader := fmt.Sprintf("ttwid=%s", ttwid)
			if c.cookies != "" {
				cookieHeader = fmt.Sprintf("ttwid=%s; %s", ttwid, c.cookies)
			}

			wssURL := connection.BuildWSSURL(c.cdnHost, room.RoomID, tz, lang, reg, c.compress)
			wsErr := connection.RunWebSocket(ctx, wssURL, cookieHeader, ua, room.RoomID, c.staleTimeout, acceptLang, c.proxy, eventCh)

			var isDeviceBlocked bool
			if wsErr != nil {
				var dbErr *connection.DeviceBlockedError
				if errors.As(wsErr, &dbErr) {
					isDeviceBlocked = true
					log.Printf("DEVICE_BLOCKED -- rotating ttwid + UA")
				} else {
					log.Printf("wss error: %v", wsErr)
				}
			}

			if ctx.Err() != nil {
				break
			}

			attempt++
			if attempt > c.maxRetries {
				log.Printf("max retries (%d) exceeded", c.maxRetries)
				break
			}

			// On DEVICE_BLOCKED: short 2s delay since we're getting a fresh
			// ttwid + UA anyway. On other errors: exponential backoff.
			var delay time.Duration
			if isDeviceBlocked {
				delay = 2 * time.Second
			} else {
				delay = time.Duration(math.Min(float64(int(1)<<attempt), 30)) * time.Second
			}

			select {
			case eventCh <- events.Event{
				Type:   events.EventReconnecting,
				RoomID: room.RoomID,
				Data:   fmt.Sprintf("attempt=%d max=%d delay=%v", attempt, c.maxRetries, delay),
			}:
			default:
			}
			log.Printf("reconnecting in %v (attempt %d/%d)", delay, attempt, c.maxRetries)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				break
			}
		}

		select {
		case eventCh <- events.Event{Type: events.EventDisconnected}:
		default:
		}
	}()

	return eventCh, nil
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
