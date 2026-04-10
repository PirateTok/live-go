package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sigiMarker = `id="__UNIVERSAL_DATA_FOR_REHYDRATION__"`

// SigiProfile holds profile data scraped from a TikTok profile page.
type SigiProfile struct {
	UserID         string
	UniqueID       string
	Nickname       string
	Bio            string
	AvatarThumb    string
	AvatarMedium   string
	AvatarLarge    string
	Verified       bool
	PrivateAccount bool
	IsOrganization bool
	RoomID         string
	BioLink        string // empty if not set
	FollowerCount  int64
	FollowingCount int64
	HeartCount     int64
	VideoCount     int64
	FriendCount    int64
}

// ScrapeProfile fetches a TikTok profile page and extracts profile data
// from the embedded SIGI JSON blob. Stateless — no caching.
func ScrapeProfile(username string, ttwid string, timeout time.Duration, userAgent string, cookies string, proxy string) (*SigiProfile, error) {
	clean := strings.ToLower(strings.TrimLeft(strings.TrimSpace(username), "@"))
	ua := userAgent
	if ua == "" {
		ua = RandomUA()
	}
	cookieHeader := buildCookieHeader(ttwid, cookies)

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("scrape profile: invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://www.tiktok.com/@%s", clean), nil)
	if err != nil {
		return nil, fmt.Errorf("scrape profile: build request: %w", err)
	}
	sLang, sReg := SystemLocale()
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Accept-Language", fmt.Sprintf("%s-%s,%s;q=0.9", sLang, sReg, sLang))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape profile: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scrape profile: read body: %w", err)
	}

	html := string(body)
	jsonStr, err := extractSigiJSON(html)
	if err != nil {
		return nil, err
	}

	var blob map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &blob); err != nil {
		return nil, &ProfileScrapeError{Reason: "JSON parse failed"}
	}

	scopeRaw, ok := blob["__DEFAULT_SCOPE__"]
	if !ok {
		return nil, &ProfileScrapeError{Reason: "missing __DEFAULT_SCOPE__"}
	}
	var scope map[string]json.RawMessage
	if err := json.Unmarshal(scopeRaw, &scope); err != nil {
		return nil, &ProfileScrapeError{Reason: "parse __DEFAULT_SCOPE__"}
	}

	detailRaw, ok := scope["webapp.user-detail"]
	if !ok {
		return nil, &ProfileScrapeError{Reason: "missing webapp.user-detail"}
	}

	var detail struct {
		StatusCode int64 `json:"statusCode"`
		UserInfo   struct {
			User  map[string]json.RawMessage `json:"user"`
			Stats map[string]json.RawMessage `json:"stats"`
		} `json:"userInfo"`
	}
	if err := json.Unmarshal(detailRaw, &detail); err != nil {
		return nil, &ProfileScrapeError{Reason: "parse user-detail"}
	}

	if detail.StatusCode != 0 {
		switch detail.StatusCode {
		case 10222:
			return nil, &ProfilePrivateError{Username: clean}
		case 10221, 10223:
			return nil, &ProfileNotFoundError{Username: clean}
		default:
			return nil, &ProfileError{Code: detail.StatusCode}
		}
	}

	if detail.UserInfo.User == nil {
		return nil, &ProfileScrapeError{Reason: "missing userInfo"}
	}

	user := detail.UserInfo.User
	stats := detail.UserInfo.Stats

	var bioLink string
	if raw, ok := user["bioLink"]; ok {
		var bl struct {
			Link string `json:"link"`
		}
		if json.Unmarshal(raw, &bl) == nil && bl.Link != "" {
			bioLink = bl.Link
		}
	}

	return &SigiProfile{
		UserID:         jsonStr_field(user, "id"),
		UniqueID:       jsonStr_field(user, "uniqueId"),
		Nickname:       jsonStr_field(user, "nickname"),
		Bio:            jsonStr_field(user, "signature"),
		AvatarThumb:    jsonStr_field(user, "avatarThumb"),
		AvatarMedium:   jsonStr_field(user, "avatarMedium"),
		AvatarLarge:    jsonStr_field(user, "avatarLarger"),
		Verified:       jsonBool_field(user, "verified"),
		PrivateAccount: jsonBool_field(user, "privateAccount"),
		IsOrganization: jsonInt_field(user, "isOrganization") != 0,
		RoomID:         jsonStr_field(user, "roomId"),
		BioLink:        bioLink,
		FollowerCount:  jsonInt_field(stats, "followerCount"),
		FollowingCount: jsonInt_field(stats, "followingCount"),
		HeartCount:     jsonInt_field(stats, "heartCount"),
		VideoCount:     jsonInt_field(stats, "videoCount"),
		FriendCount:    jsonInt_field(stats, "friendCount"),
	}, nil
}

func extractSigiJSON(html string) (string, error) {
	markerPos := strings.Index(html, sigiMarker)
	if markerPos < 0 {
		return "", &ProfileScrapeError{Reason: "SIGI script tag not found in HTML"}
	}
	gtPos := strings.IndexByte(html[markerPos:], '>')
	if gtPos < 0 {
		return "", &ProfileScrapeError{Reason: "no > after SIGI marker"}
	}
	jsonStart := markerPos + gtPos + 1
	scriptEnd := strings.Index(html[jsonStart:], "</script>")
	if scriptEnd < 0 {
		return "", &ProfileScrapeError{Reason: "no </script> after SIGI JSON"}
	}
	jsonStr := html[jsonStart : jsonStart+scriptEnd]
	if jsonStr == "" {
		return "", &ProfileScrapeError{Reason: "empty SIGI JSON blob"}
	}
	return jsonStr, nil
}

func buildCookieHeader(ttwid string, cookies string) string {
	if cookies == "" {
		return fmt.Sprintf("ttwid=%s", ttwid)
	}
	parts := strings.Split(cookies, "; ")
	var filtered []string
	for _, p := range parts {
		if !strings.HasPrefix(p, "ttwid=") {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return fmt.Sprintf("ttwid=%s", ttwid)
	}
	return fmt.Sprintf("ttwid=%s; %s", ttwid, strings.Join(filtered, "; "))
}

func jsonStr_field(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func jsonBool_field(m map[string]json.RawMessage, key string) bool {
	raw, ok := m[key]
	if !ok {
		return false
	}
	var b bool
	if json.Unmarshal(raw, &b) != nil {
		return false
	}
	return b
}

func jsonInt_field(m map[string]json.RawMessage, key string) int64 {
	raw, ok := m[key]
	if !ok {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) != nil {
		return 0
	}
	return n
}
