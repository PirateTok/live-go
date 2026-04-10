package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RoomIDResult holds the result of a successful room ID resolution.
type RoomIDResult struct {
	RoomID string
}

// RoomInfo holds optional metadata fetched from the room/info endpoint.
type RoomInfo struct {
	Title     string
	Viewers   int64
	Likes     int64
	TotalUser int64
	StreamURL *StreamURLs
}

// StreamURLs holds the FLV stream URLs by quality tier.
type StreamURLs struct {
	FlvOrigin string
	FlvHD     string
	FlvSD     string
	FlvLD     string
	FlvAudio  string
}

// Error types for room resolution.
type UserNotFoundError struct{ Username string }
type HostNotOnlineError struct{ Username string }
type TikTokBlockedError struct{ StatusCode int }
type TikTokAPIError struct{ Code int64 }

func (e *UserNotFoundError) Error() string  { return fmt.Sprintf("user %q does not exist", e.Username) }
func (e *HostNotOnlineError) Error() string { return fmt.Sprintf("user %q is not currently live", e.Username) }
func (e *TikTokBlockedError) Error() string { return fmt.Sprintf("tiktok blocked (HTTP %d)", e.StatusCode) }
func (e *TikTokAPIError) Error() string     { return fmt.Sprintf("tiktok API error: statusCode=%d", e.Code) }

// Profile errors
type ProfilePrivateError struct{ Username string }
type ProfileNotFoundError struct{ Username string }
type ProfileScrapeError struct{ Reason string }
type ProfileError struct{ Code int64 }

func (e *ProfilePrivateError) Error() string  { return fmt.Sprintf("profile is private: @%s", e.Username) }
func (e *ProfileNotFoundError) Error() string { return fmt.Sprintf("profile not found: @%s", e.Username) }
func (e *ProfileScrapeError) Error() string   { return fmt.Sprintf("failed to scrape profile: %s", e.Reason) }
func (e *ProfileError) Error() string         { return fmt.Sprintf("profile fetch error: statusCode=%d", e.Code) }

// AgeRestrictedError is returned when room info fails on 18+ rooms without cookies.
type AgeRestrictedError struct{}

func (e *AgeRestrictedError) Error() string {
	return "age-restricted stream: 18+ room — pass session cookies to FetchRoomInfo()"
}

// CheckOnline resolves a TikTok username to a room ID via the JSON API.
// Pass empty language/region to auto-detect from system locale.
func CheckOnline(username string, timeout time.Duration, language string, region string) (*RoomIDResult, error) {
	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
	lang, reg := resolveLocale(language, region)
	browserLang := lang + "-" + reg
	url := fmt.Sprintf(
		"https://www.tiktok.com/api-live/user/room?aid=1988&app_name=tiktok_web"+
			"&device_platform=web_pc&app_language=%s&browser_language=%s&region=%s"+
			"&user_is_login=false&sourceType=54&staleTime=600000&uniqueId=%s",
		lang, browserLang, reg, username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("check online: build request: %w", err)
	}
	req.Header.Set("User-Agent", RandomUA())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check online: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return nil, &TikTokBlockedError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("check online: read body: %w", err)
	}

	var result struct {
		StatusCode int64 `json:"statusCode"`
		Data       struct {
			User struct {
				RoomID string `json:"roomId"`
				Status int    `json:"status"`
			} `json:"user"`
			LiveRoom struct {
				Status int `json:"status"`
			} `json:"liveRoom"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &TikTokBlockedError{StatusCode: resp.StatusCode}
	}

	if result.StatusCode == 19881007 {
		return nil, &UserNotFoundError{Username: username}
	}
	if result.StatusCode != 0 {
		return nil, &TikTokAPIError{Code: result.StatusCode}
	}

	roomID := result.Data.User.RoomID
	if roomID == "" || roomID == "0" {
		return nil, &HostNotOnlineError{Username: username}
	}
	if result.Data.LiveRoom.Status != 2 && result.Data.User.Status != 2 {
		return nil, &HostNotOnlineError{Username: username}
	}

	return &RoomIDResult{RoomID: roomID}, nil
}

// FetchRoomInfo fetches optional room metadata. Cookies needed for 18+ rooms.
// Pass empty language/region to auto-detect from system locale.
func FetchRoomInfo(roomID string, timeout time.Duration, cookies string, language string, region string) (*RoomInfo, error) {
	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
	tz := strings.ReplaceAll(SystemTimezone(), "/", "%2F")
	lang, reg := resolveLocale(language, region)
	browserLang := lang + "-" + reg
	url := fmt.Sprintf(
		"https://webcast.tiktok.com/webcast/room/info/?aid=1988&app_name=tiktok_web"+
			"&device_platform=web_pc&app_language=%s&browser_language=%s"+
			"&browser_name=Mozilla&browser_online=true&browser_platform=Linux+x86_64"+
			"&cookie_enabled=true&screen_height=1080&screen_width=1920"+
			"&tz_name=%s&webcast_language=%s&room_id=%s",
		lang, browserLang, tz, lang, roomID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("room info: build request: %w", err)
	}
	req.Header.Set("User-Agent", RandomUA())
	req.Header.Set("Referer", "https://www.tiktok.com/")
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("room info: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("room info: read body: %w", err)
	}

	var raw struct {
		StatusCode int64 `json:"status_code"`
		Data       struct {
			Title     string `json:"title"`
			LikeCount int64  `json:"like_count"`
			UserCount int64  `json:"user_count"`
			Stats     struct {
				LikeCount int64 `json:"like_count"`
				TotalUser int64 `json:"total_user"`
			} `json:"stats"`
			StreamURL json.RawMessage `json:"stream_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("room info: parse JSON: %w", err)
	}

	if raw.StatusCode == 4003110 {
		return nil, &AgeRestrictedError{}
	}
	if raw.StatusCode != 0 {
		return nil, fmt.Errorf("room info: status_code=%d", raw.StatusCode)
	}

	info := &RoomInfo{
		Title:     raw.Data.Title,
		Likes:     raw.Data.Stats.LikeCount,
		Viewers:   raw.Data.UserCount,
		TotalUser: raw.Data.Stats.TotalUser,
	}

	if len(raw.Data.StreamURL) > 0 {
		info.StreamURL = parseStreamURLs(raw.Data.StreamURL)
	}
	return info, nil
}

func parseStreamURLs(data json.RawMessage) *StreamURLs {
	var raw struct {
		FlvPullURL map[string]string `json:"flv_pull_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	if len(raw.FlvPullURL) == 0 {
		return nil
	}

	return &StreamURLs{
		FlvOrigin: raw.FlvPullURL["FULL_HD1"],
		FlvHD:     raw.FlvPullURL["HD1"],
		FlvSD:     raw.FlvPullURL["SD1"],
		FlvLD:     raw.FlvPullURL["SD2"],
		FlvAudio:  raw.FlvPullURL["AUDIO"],
	}
}

func resolveLocale(language string, region string) (string, string) {
	lang := language
	reg := region
	if lang == "" || reg == "" {
		sysLang, sysReg := SystemLocale()
		if lang == "" {
			lang = sysLang
		}
		if reg == "" {
			reg = sysReg
		}
	}
	return lang, reg
}
