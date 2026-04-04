package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	tthttp "github.com/PirateTok/live-go/http"
)

// FetchTTWID performs an unauthenticated GET to tiktok.com and extracts
// the ttwid cookie from the Set-Cookie response header.
// The userAgent parameter overrides the default random UA when non-empty.
func FetchTTWID(timeout time.Duration, userAgent string) (string, error) {
	client := &http.Client{
		Timeout: timeout,
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
