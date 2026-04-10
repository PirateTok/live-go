//go:build integration

package golive

import (
	"errors"
	"os"
	"testing"
	"time"

	tthttp "github.com/PirateTok/live-go/http"
)

// syntheticNonexistentUser is hardcoded per-lib. TikTok must return user-not-found for this probe.
const syntheticNonexistentUser = "piratetok_go_nf_7a3c9e2f1b8d4a6c0e5f3a2b1d9c8e7"

const httpTimeout = 25 * time.Second

// TestCheckOnline_LiveUser — H1: check_online on a live user returns a non-empty room ID.
func TestCheckOnline_LiveUser(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}

	result, err := tthttp.CheckOnline(user, httpTimeout, "", "", "")
	if err != nil {
		t.Fatalf("CheckOnline(%q) returned error: %v", user, err)
	}
	if result == nil {
		t.Fatalf("CheckOnline(%q) returned nil result", user)
	}
	if result.RoomID == "" {
		t.Fatalf("CheckOnline(%q) returned empty room ID", user)
	}
	if result.RoomID == "0" {
		t.Fatalf("CheckOnline(%q) returned zero room ID", user)
	}
	t.Logf("[integration test online] @%s is live in room %s", user, result.RoomID)
}

// TestCheckOnline_OfflineUser — H2: check_online on an offline user returns HostNotOnlineError.
func TestCheckOnline_OfflineUser(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_OFFLINE_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_OFFLINE_USER to a known-offline TikTok username to run this test")
	}

	_, err := tthttp.CheckOnline(user, httpTimeout, "", "", "")
	if err == nil {
		t.Fatalf("CheckOnline(%q) expected HostNotOnlineError but got nil error (user appears live)", user)
	}

	var notOnline *tthttp.HostNotOnlineError
	if !errors.As(err, &notOnline) {
		t.Fatalf("CheckOnline(%q) returned wrong error type: want *HostNotOnlineError, got %T: %v", user, err, err)
	}
	t.Logf("[integration test offline] @%s correctly identified as not online", user)
}

// TestCheckOnline_NonexistentUser — H3: check_online on a nonexistent user returns UserNotFoundError.
// Gated on PIRATETOK_LIVE_TEST_HTTP=1 since it calls TikTok's API with a synthetic username.
func TestCheckOnline_NonexistentUser(t *testing.T) {
	flag := os.Getenv("PIRATETOK_LIVE_TEST_HTTP")
	if flag != "1" && flag != "true" && flag != "yes" {
		t.Skip("set PIRATETOK_LIVE_TEST_HTTP=1 to enable the not-found probe (calls TikTok API with a synthetic username)")
	}

	_, err := tthttp.CheckOnline(syntheticNonexistentUser, httpTimeout, "", "", "")
	if err == nil {
		t.Fatalf("CheckOnline(%q) expected UserNotFoundError but got nil (synthetic user exists?)", syntheticNonexistentUser)
	}

	var notFound *tthttp.UserNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("CheckOnline(%q) returned wrong error type: want *UserNotFoundError, got %T: %v",
			syntheticNonexistentUser, err, err)
	}
	if notFound.Username != syntheticNonexistentUser {
		t.Errorf("UserNotFoundError.Username: got %q, want %q", notFound.Username, syntheticNonexistentUser)
	}
	t.Logf("[integration test not-found] synthetic user correctly identified as not found")
}

// TestFetchRoomInfo_LiveRoom — H4: fetch_room_info returns room info with viewers >= 0.
func TestFetchRoomInfo_LiveRoom(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}

	room, err := tthttp.CheckOnline(user, httpTimeout, "", "", "")
	if err != nil {
		t.Fatalf("CheckOnline(%q) failed: %v", user, err)
	}

	cookies := os.Getenv("PIRATETOK_LIVE_TEST_COOKIES")
	info, err := tthttp.FetchRoomInfo(room.RoomID, httpTimeout, cookies, "", "", "")
	if err != nil {
		t.Fatalf("FetchRoomInfo(%q) failed: %v", room.RoomID, err)
	}
	if info == nil {
		t.Fatalf("FetchRoomInfo(%q) returned nil", room.RoomID)
	}
	if info.Viewers < 0 {
		t.Errorf("FetchRoomInfo viewers < 0: got %d", info.Viewers)
	}
	t.Logf("[integration test room-info] room=%s title=%q viewers=%d", room.RoomID, info.Title, info.Viewers)
}
