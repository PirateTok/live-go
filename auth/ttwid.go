package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	tthttp "github.com/PirateTok/live-go/http"
)

// FetchTTWID performs an unauthenticated GET to tiktok.com and extracts
// the ttwid cookie from the Set-Cookie response header.
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

	req, err := http.NewRequest("GET", "https://www.tiktok.com/", nil)
	if err != nil {
		return "", fmt.Errorf("ttwid: build request: %w", err)
	}

	ua := userAgent
	if ua == "" {
		ua = tthttp.RandomUA()
	}
	req.Header.Set("User-Agent", ua)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ttwid: GET tiktok.com: %w", err)
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "ttwid" {
			value := strings.TrimSpace(cookie.Value)
			if value == "" {
				return "", fmt.Errorf("ttwid: empty ttwid cookie value")
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("ttwid: no ttwid cookie in response (status %d)", resp.StatusCode)
}
