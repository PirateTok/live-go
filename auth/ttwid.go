package auth

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	tthttp "github.com/PirateTok/live-go/http"
)

const tiktokURL = "https://www.tiktok.com/"

// TikTok only sets ttwid on ~1 in 5-8 anonymous GETs — retry when it's absent.
const (
	FetchAttempts = 8
	RetryDelay    = 750 * time.Millisecond
)

// FetchTTWID performs an unauthenticated GET to tiktok.com and extracts
// the ttwid cookie from the Set-Cookie response header, retrying up to
// FetchAttempts times when the cookie is missing. Transport errors return
// immediately.
// The userAgent parameter overrides the default random UA when non-empty.
// The proxy parameter sets an HTTP/HTTPS proxy when non-empty.
func FetchTTWID(timeout time.Duration, userAgent string, proxy string) (string, error) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return "", fmt.Errorf("ttwid: invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ua := userAgent
	if ua == "" {
		ua = tthttp.RandomUA()
	}
	return fetchTTWID(client, tiktokURL, ua, FetchAttempts, RetryDelay)
}

func fetchTTWID(client *http.Client, target string, ua string, attempts int, delay time.Duration) (string, error) {
	for attempt := 1; ; attempt++ {
		ttwid, status, err := fetchOnce(client, target, ua)
		if err != nil {
			return "", err
		}
		if ttwid != "" {
			return ttwid, nil
		}
		if attempt >= attempts {
			return "", fmt.Errorf("ttwid: no ttwid cookie after %d attempts (last status %d)", attempt, status)
		}
		log.Printf("ttwid: no cookie in response (attempt %d, status %d), retrying", attempt, status)
		time.Sleep(delay)
	}
}

// fetchOnce returns an empty ttwid (and no error) when the response simply lacks the cookie.
func fetchOnce(client *http.Client, target string, ua string) (string, int, error) {
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return "", 0, fmt.Errorf("ttwid: build request: %w", err)
	}
	req.Header.Set("User-Agent", ua)

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("ttwid: GET tiktok.com: %w", err)
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "ttwid" {
			return strings.TrimSpace(cookie.Value), resp.StatusCode, nil
		}
	}
	return "", resp.StatusCode, nil
}
