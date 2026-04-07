package http

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultTTL      = 5 * time.Minute
	ttwidTimeout    = 10 * time.Second
	scrapeTimeout   = 15 * time.Second
)

type cacheEntry struct {
	profile    *SigiProfile
	err        error
	insertedAt time.Time
}

// ProfileCache is a thread-safe cached profile fetcher.
type ProfileCache struct {
	mu        sync.Mutex
	entries   map[string]*cacheEntry
	ttwid     string
	ttl       time.Duration
	userAgent string
	cookies   string
}

// NewProfileCache creates a new cache with default TTL (5 minutes).
func NewProfileCache() *ProfileCache {
	return &ProfileCache{
		entries: make(map[string]*cacheEntry),
		ttl:     defaultTTL,
	}
}

// WithTTL sets the cache TTL.
func (c *ProfileCache) WithTTL(ttl time.Duration) *ProfileCache {
	c.ttl = ttl
	return c
}

// WithUserAgent sets a fixed user agent for all requests.
func (c *ProfileCache) WithUserAgent(ua string) *ProfileCache {
	c.userAgent = ua
	return c
}

// WithCookies sets session cookies for login-required profiles.
func (c *ProfileCache) WithCookies(cookies string) *ProfileCache {
	c.cookies = cookies
	return c
}

// Fetch returns a cached profile if valid, otherwise scrapes and caches.
func (c *ProfileCache) Fetch(username string) (*SigiProfile, error) {
	key := normalizeKey(username)

	c.mu.Lock()
	if entry, ok := c.entries[key]; ok && time.Since(entry.insertedAt) < c.ttl {
		c.mu.Unlock()
		if entry.err != nil {
			return nil, entry.err
		}
		return entry.profile, nil
	}
	c.mu.Unlock()

	ttwid, err := c.ensureTTWID()
	if err != nil {
		return nil, err
	}

	profile, scrapeErr := ScrapeProfile(key, ttwid, scrapeTimeout, c.userAgent, c.cookies)

	c.mu.Lock()
	if scrapeErr != nil {
		if isNegativeCacheable(scrapeErr) {
			c.entries[key] = &cacheEntry{err: scrapeErr, insertedAt: time.Now()}
		}
		c.mu.Unlock()
		return nil, scrapeErr
	}
	c.entries[key] = &cacheEntry{profile: profile, insertedAt: time.Now()}
	c.mu.Unlock()
	return profile, nil
}

// Cached returns a cached profile without fetching. Returns nil on miss or expiry.
func (c *ProfileCache) Cached(username string) *SigiProfile {
	key := normalizeKey(username)
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.insertedAt) >= c.ttl || entry.err != nil {
		return nil
	}
	return entry.profile
}

// Invalidate removes one entry from the cache.
func (c *ProfileCache) Invalidate(username string) {
	key := normalizeKey(username)
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *ProfileCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

func (c *ProfileCache) ensureTTWID() (string, error) {
	c.mu.Lock()
	if c.ttwid != "" {
		ttwid := c.ttwid
		c.mu.Unlock()
		return ttwid, nil
	}
	c.mu.Unlock()

	ttwid, err := fetchTTWIDInternal(ttwidTimeout, c.userAgent)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.ttwid = ttwid
	c.mu.Unlock()
	return ttwid, nil
}

// fetchTTWIDInternal fetches a fresh ttwid without importing the auth package.
// Duplicates the minimal logic to avoid the import cycle.
func fetchTTWIDInternal(timeout time.Duration, userAgent string) (string, error) {
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
		ua = RandomUA()
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

func normalizeKey(username string) string {
	s := username
	for len(s) > 0 && (s[0] == ' ' || s[0] == '@') {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func isNegativeCacheable(err error) bool {
	switch err.(type) {
	case *ProfilePrivateError, *ProfileNotFoundError, *ProfileError:
		return true
	}
	return false
}
