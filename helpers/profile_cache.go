package helpers

import (
	"strings"
	"sync"
	"time"

	"github.com/PirateTok/live-go/auth"
	tthttp "github.com/PirateTok/live-go/http"
)

const (
	defaultTTL    = 5 * time.Minute
	ttwidTimeout  = 10 * time.Second
	scrapeTimeout = 15 * time.Second
)

type cacheEntry struct {
	profile    *tthttp.SigiProfile
	err        error
	insertedAt time.Time
}

// ProfileCache is a thread-safe cached profile fetcher.
type ProfileCache struct {
	mu        sync.Mutex
	entries   map[string]*cacheEntry
	ttwid     string
	ttl       time.Duration
	proxy     string
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

// WithProxy sets a proxy URL for all HTTP requests.
func (c *ProfileCache) WithProxy(proxy string) *ProfileCache {
	c.proxy = proxy
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
func (c *ProfileCache) Fetch(username string) (*tthttp.SigiProfile, error) {
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

	profile, scrapeErr := tthttp.ScrapeProfile(key, ttwid, scrapeTimeout, c.userAgent, c.cookies, c.proxy)

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
func (c *ProfileCache) Cached(username string) *tthttp.SigiProfile {
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

	ttwid, err := auth.FetchTTWID(ttwidTimeout, c.userAgent, c.proxy)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.ttwid = ttwid
	c.mu.Unlock()
	return ttwid, nil
}

func normalizeKey(username string) string {
	s := strings.TrimSpace(username)
	s = strings.TrimLeft(s, "@")
	return strings.ToLower(s)
}

func isNegativeCacheable(err error) bool {
	switch err.(type) {
	case *tthttp.ProfilePrivateError, *tthttp.ProfileNotFoundError, *tthttp.ProfileError:
		return true
	}
	return false
}
