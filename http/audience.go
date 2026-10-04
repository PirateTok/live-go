package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const statusSessionRequired = 20003

// SessionRequiredError is returned by login-gated endpoints when no valid
// session cookies were passed.
type SessionRequiredError struct{ Reason string }

func (e *SessionRequiredError) Error() string { return "session required: " + e.Reason }

// RoomAudience is the full viewer roster of a live room.
type RoomAudience struct {
	Total     int64
	Anonymous int64
	Viewers   []AudienceViewer
	RawJSON   string
}

// AudienceViewer is one named viewer in the roster.
type AudienceViewer struct {
	Rank          int64
	Score         int64
	UserID        string
	Username      string
	Nickname      string
	SecUID        string
	AvatarURL     string // empty when TikTok sent none
	FollowerCount int64
	Verified      bool
	IsFollower    bool // follows the streamer
	IsFollowing   bool // the streamer follows them
	IsSubscriber  bool
}

// FetchRoomAudience fetches the full audience roster: every named viewer
// currently in the room — the whole viewer panel, not just the top-3 box (for
// that, see WebcastRoomUserSeqMessage.TopViewers, which needs no cookies).
//
// TikTok gates this endpoint behind a login: pass session cookies
// ("sessionid=xxx; sid_tt=xxx") or you get *SessionRequiredError. No ttwid,
// msToken, or signing needed.
//
// anchorID is the streamer's user ID (RoomIDResult.AnchorID). Pass "" to
// auto-resolve it from room info (one extra request).
func FetchRoomAudience(roomID string, anchorID string, timeout time.Duration, cookies string, language string, region string, proxy string) (*RoomAudience, error) {
	if anchorID == "" {
		resolved, err := resolveAnchorID(roomID, timeout, cookies, language, region, proxy)
		if err != nil {
			return nil, err
		}
		anchorID = resolved
	}

	transport, err := buildTransport(proxy)
	if err != nil {
		return nil, fmt.Errorf("online audience: %w", err)
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	lang, reg := resolveLocale(language, region)
	target := fmt.Sprintf(
		"https://webcast.tiktok.com/webcast/ranklist/online_audience/?aid=1988&app_name=tiktok_web"+
			"&device_platform=web_pc&app_language=%s&browser_language=%s-%s&channel=tiktok_web"+
			"&room_id=%s&anchor_id=%s",
		lang, lang, reg, roomID, anchorID)

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return nil, fmt.Errorf("online audience: build request: %w", err)
	}
	req.Header.Set("User-Agent", RandomUA())
	req.Header.Set("Referer", "https://www.tiktok.com/")
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("online audience: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("online audience: read body: %w", err)
	}
	return parseRoomAudience(body, resp.StatusCode)
}

func resolveAnchorID(roomID string, timeout time.Duration, cookies string, language string, region string, proxy string) (string, error) {
	info, err := FetchRoomInfo(roomID, timeout, cookies, language, region, proxy)
	if err != nil {
		return "", err
	}
	var raw struct {
		Data struct {
			Owner struct {
				IDStr string `json:"id_str"`
			} `json:"owner"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(info.RawJSON), &raw); err != nil {
		return "", fmt.Errorf("room info: parse owner: %w", err)
	}
	if raw.Data.Owner.IDStr == "" {
		return "", fmt.Errorf("no owner id in room info")
	}
	return raw.Data.Owner.IDStr, nil
}

type audienceUser struct {
	ID          json.RawMessage `json:"id"`
	IDStr       string          `json:"id_str"`
	DisplayID   string          `json:"display_id"`
	Nickname    string          `json:"nickname"`
	SecUID      string          `json:"sec_uid"`
	AvatarThumb struct {
		URLList []string `json:"url_list"`
	} `json:"avatar_thumb"`
	FollowInfo struct {
		FollowerCount int64 `json:"follower_count"`
	} `json:"follow_info"`
	Verified    bool `json:"verified"`
	IsFollower  bool `json:"is_follower"`
	IsFollowing bool `json:"is_following"`
	IsSubscribe bool `json:"is_subscribe"`
}

func parseRoomAudience(body []byte, httpStatus int) (*RoomAudience, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty response from online_audience (http %d)", httpStatus)
	}
	var raw struct {
		StatusCode *int64 `json:"status_code"`
		Data       *struct {
			Message   string `json:"message"`
			Total     int64  `json:"total"`
			Anonymous int64  `json:"anonymous"`
			Ranks     []struct {
				Rank  int64         `json:"rank"`
				Score int64         `json:"score"`
				User  *audienceUser `json:"user"`
			} `json:"ranks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("online audience: parse JSON: %w", err)
	}
	if raw.StatusCode == nil {
		return nil, fmt.Errorf("no status_code in online_audience response")
	}
	code := *raw.StatusCode
	if code == statusSessionRequired {
		return nil, &SessionRequiredError{Reason: "audience roster needs login — pass session cookies to FetchRoomAudience()"}
	}
	if code != 0 {
		msg := ""
		if raw.Data != nil {
			msg = raw.Data.Message
		}
		return nil, fmt.Errorf("online_audience status_code=%d %s", code, msg)
	}
	if raw.Data == nil {
		return nil, fmt.Errorf("missing 'data' in online_audience")
	}

	audience := &RoomAudience{
		Total:     raw.Data.Total,
		Anonymous: raw.Data.Anonymous,
		Viewers:   make([]AudienceViewer, 0, len(raw.Data.Ranks)),
		RawJSON:   string(body),
	}
	for _, rank := range raw.Data.Ranks {
		if rank.User == nil {
			continue
		}
		audience.Viewers = append(audience.Viewers, audienceViewer(rank.Rank, rank.Score, rank.User))
	}
	return audience, nil
}

func audienceViewer(rank int64, score int64, u *audienceUser) AudienceViewer {
	userID := u.IDStr
	if userID == "" {
		userID = idString(u.ID)
	}
	avatar := ""
	if len(u.AvatarThumb.URLList) > 0 {
		avatar = u.AvatarThumb.URLList[0]
	}
	return AudienceViewer{
		Rank:          rank,
		Score:         score,
		UserID:        userID,
		Username:      u.DisplayID,
		Nickname:      u.Nickname,
		SecUID:        u.SecUID,
		AvatarURL:     avatar,
		FollowerCount: u.FollowInfo.FollowerCount,
		Verified:      u.Verified,
		IsFollower:    u.IsFollower,
		IsFollowing:   u.IsFollowing,
		IsSubscriber:  u.IsSubscribe,
	}
}
