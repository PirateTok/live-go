package connection

import (
	"fmt"
	"math/rand"
	"net/url"
)

const defaultCDNHost = "webcast-ws.tiktok.com"

// BuildWSSURL constructs the TikTok Live WebSocket URL for a given room.
// The timezone parameter should be an IANA timezone name (e.g. "Europe/London").
func BuildWSSURL(cdnHost string, roomID string, timezone string, language string, region string) string {
	if cdnHost == "" {
		cdnHost = defaultCDNHost
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if language == "" {
		language = "en"
	}
	if region == "" {
		region = "US"
	}

	lastRtt := 100.0 + rand.Float64()*100.0
	browserLanguage := language + "-" + region

	params := url.Values{
		"version_code":          {"180800"},
		"device_platform":       {"web"},
		"cookie_enabled":        {"true"},
		"screen_width":          {"1920"},
		"screen_height":         {"1080"},
		"browser_language":      {browserLanguage},
		"browser_platform":      {"Linux x86_64"},
		"browser_name":          {"Mozilla"},
		"browser_version":       {"5.0 (X11)"},
		"browser_online":        {"true"},
		"tz_name":               {timezone},
		"app_name":              {"tiktok_web"},
		"sup_ws_ds_opt":         {"1"},
		"update_version_code":   {"2.0.0"},
		"compress":              {"gzip"},
		"webcast_language":      {language},
		"ws_direct":             {"1"},
		"aid":                   {"1988"},
		"live_id":               {"12"},
		"app_language":          {language},
		"client_enter":          {"1"},
		"room_id":               {roomID},
		"identity":              {"audience"},
		"history_comment_count": {"6"},
		"last_rtt":              {fmt.Sprintf("%.3f", lastRtt)},
		"heartbeat_duration":    {"10000"},
		"resp_content_type":     {"protobuf"},
		"did_rule":              {"3"},
	}

	return fmt.Sprintf("wss://%s/webcast/im/ws_proxy/ws_reuse_supplement/?%s", cdnHost, params.Encode())
}
