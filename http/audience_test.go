package http

import (
	"errors"
	"strings"
	"testing"
)

const audienceOK = `{
  "status_code": 0,
  "data": {
    "total": 1234,
    "anonymous": 56,
    "ranks": [
      {"rank": 1, "score": 900, "user": {
        "id": 111, "id_str": "7000000000000000111", "display_id": "viewer_one",
        "nickname": "Viewer One", "sec_uid": "MS4w-one",
        "avatar_thumb": {"url_list": ["https://p16.example/a.webp", "https://p19.example/a.webp"]},
        "follow_info": {"follower_count": 42},
        "verified": true, "is_follower": true, "is_following": false, "is_subscribe": true}},
      {"rank": 2, "score": 10},
      {"rank": 3, "score": 5, "user": {"id": 333, "display_id": "viewer_three", "nickname": "V3"}}
    ]
  }
}`

func TestParseRoomAudience(t *testing.T) {
	a, err := parseRoomAudience([]byte(audienceOK), 200)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if a.Total != 1234 || a.Anonymous != 56 || len(a.Viewers) != 2 || a.RawJSON != audienceOK {
		t.Fatalf("total=%d anon=%d viewers=%d", a.Total, a.Anonymous, len(a.Viewers))
	}
	one := a.Viewers[0]
	want := AudienceViewer{
		Rank: 1, Score: 900, UserID: "7000000000000000111", Username: "viewer_one",
		Nickname: "Viewer One", SecUID: "MS4w-one", AvatarURL: "https://p16.example/a.webp",
		FollowerCount: 42, Verified: true, IsFollower: true, IsFollowing: false, IsSubscriber: true,
	}
	if one != want {
		t.Fatalf("viewer one = %+v", one)
	}
	three := a.Viewers[1]
	if three.UserID != "333" || three.AvatarURL != "" || three.FollowerCount != 0 {
		t.Fatalf("viewer three = %+v", three)
	}
}

func TestParseRoomAudienceSessionRequired(t *testing.T) {
	_, err := parseRoomAudience([]byte(`{"status_code":20003,"data":{"message":"login"}}`), 200)
	var sr *SessionRequiredError
	if !errors.As(err, &sr) || !strings.Contains(err.Error(), "session cookies") {
		t.Fatalf("err = %v, want *SessionRequiredError", err)
	}
}

func TestParseRoomAudienceOtherStatus(t *testing.T) {
	_, err := parseRoomAudience([]byte(`{"status_code":10011,"data":{"message":"room gone"}}`), 200)
	if err == nil || !strings.Contains(err.Error(), "status_code=10011 room gone") {
		t.Fatalf("err = %v", err)
	}
	var sr *SessionRequiredError
	if errors.As(err, &sr) {
		t.Fatal("non-20003 must not be SessionRequired")
	}
}

func TestParseRoomAudienceInvalid(t *testing.T) {
	if _, err := parseRoomAudience([]byte(`{"data":{}}`), 200); err == nil {
		t.Fatal("missing status_code must fail")
	}
	_, err := parseRoomAudience(nil, 403)
	if err == nil || !strings.Contains(err.Error(), "http 403") {
		t.Fatalf("empty body err = %v", err)
	}
}

func TestIDString(t *testing.T) {
	if idString([]byte(`"7001"`)) != "7001" || idString([]byte(`7001`)) != "7001" || idString(nil) != "" {
		t.Fatal("idString mismatch")
	}
}
