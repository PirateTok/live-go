//go:build integration

package golive

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/PirateTok/live-go/events"
	pb "github.com/PirateTok/live-go/proto"
)

// wssTestClient creates a client pre-configured for integration smoke tests.
func wssTestClient(username string) *Client {
	return NewClient(username).
		CdnEU().
		Timeout(15 * time.Second).
		MaxRetries(5).
		StaleTimeout(45 * time.Second)
}

// awaitWssEvent connects to a live room and waits for an event that satisfies
// the predicate. Returns when the first matching event arrives or the timeout
// is exceeded.
//
// The worker goroutine running Connect is joined before returning, and the
// context is always cancelled so there are no leaked goroutines.
func awaitWssEvent(
	t *testing.T,
	username string,
	awaitTimeout time.Duration,
	predicate func(events.Event) bool,
	failureMessage string,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := wssTestClient(username)

	eventCh, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect(%q) failed immediately: %v", username, err)
	}

	hit := make(chan struct{}, 1)
	workerDone := make(chan error, 1)

	// drain goroutine — reads events, signals hit, and drains until channel closes
	go func() {
		var connectErr error
		for evt := range eventCh {
			if evt.Type == events.EventDisconnected {
				// channel will close after this; capture any non-nil data as error
				if evt.Data != nil {
					if e, ok := evt.Data.(error); ok {
						connectErr = e
					}
				}
				continue
			}
			if predicate(evt) {
				select {
				case hit <- struct{}{}:
				default:
				}
			}
		}
		workerDone <- connectErr
	}()

	timer := time.NewTimer(awaitTimeout)
	defer timer.Stop()

	select {
	case <-hit:
		// success — cancel context to trigger clean shutdown
		cancel()
	case <-timer.C:
		cancel()
		// drain workerDone so goroutine doesn't leak
		<-workerDone
		t.Fatalf("%s", failureMessage)
	}

	// wait for the worker goroutine to finish after cancel
	select {
	case workerErr := <-workerDone:
		if workerErr != nil {
			t.Errorf("connect worker returned error after cancel: %v", workerErr)
		}
	case <-time.After(30 * time.Second):
		t.Error("connect worker did not exit within 30s after context cancel")
	}
}

// TestConnect_ReceivesTrafficBeforeTimeout — W1: any event arrives within 90s.
func TestConnect_ReceivesTrafficBeforeTimeout(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 90*time.Second, func(evt events.Event) bool {
		switch evt.Type {
		case events.EventRoomUserSeq, events.EventMember, events.EventChat,
			events.EventLike, events.EventControl, events.EventConnected:
			return true
		}
		return false
	}, "no room traffic within 90s (quiet stream or block)")
}

// TestConnect_ReceivesChatBeforeTimeout — W2: chat event arrives within 120s.
func TestConnect_ReceivesChatBeforeTimeout(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 120*time.Second, func(evt events.Event) bool {
		if evt.Type != events.EventChat {
			return false
		}
		msg, ok := evt.Data.(*pb.WebcastChatMessage)
		if !ok || msg == nil {
			return false
		}
		nick := "?"
		if msg.User != nil {
			nick = msg.User.Nickname
		}
		t.Logf("[integration test chat] %s: %s", nick, msg.Content)
		return msg.Content != ""
	}, "no chat message within 120s (quiet stream or block)")
}

// TestConnect_ReceivesGiftBeforeTimeout — W3: gift event arrives within 180s.
func TestConnect_ReceivesGiftBeforeTimeout(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 180*time.Second, func(evt events.Event) bool {
		if evt.Type != events.EventGift {
			return false
		}
		msg, ok := evt.Data.(*pb.WebcastGiftMessage)
		if !ok || msg == nil {
			return false
		}
		nick := "?"
		if msg.User != nil {
			nick = msg.User.Nickname
		}
		giftName := "?"
		if msg.Gift != nil {
			giftName = msg.Gift.Name
		}
		t.Logf("[integration test gift] %s -> %s x%d", nick, giftName, msg.RepeatCount)
		return true
	}, "no gift within 180s (quiet stream or no gifts — try a busier stream)")
}

// TestConnect_ReceivesLikeBeforeTimeout — W4: like event arrives within 120s.
func TestConnect_ReceivesLikeBeforeTimeout(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 120*time.Second, func(evt events.Event) bool {
		if evt.Type != events.EventLike {
			return false
		}
		msg, ok := evt.Data.(*pb.WebcastLikeMessage)
		if !ok || msg == nil {
			return false
		}
		nick := "?"
		if msg.User != nil {
			nick = msg.User.Nickname
		}
		t.Logf("[integration test like] %s count=%d total=%d", nick, msg.Count, msg.Total)
		return true
	}, "no like within 120s (quiet stream or block)")
}

// TestConnect_ReceivesJoinBeforeTimeout — W5: join (sub-routed from MemberMessage) arrives within 150s.
func TestConnect_ReceivesJoinBeforeTimeout(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 150*time.Second, func(evt events.Event) bool {
		if evt.Type != events.EventJoin {
			return false
		}
		msg, ok := evt.Data.(*pb.WebcastMemberMessage)
		if !ok || msg == nil {
			return false
		}
		nick := "?"
		if msg.User != nil {
			nick = msg.User.Nickname
		}
		t.Logf("[integration test join] %s", nick)
		return true
	}, "no join within 150s (try a busier stream)")
}

// TestConnect_ReceivesFollowBeforeTimeout — W6: follow (sub-routed from SocialMessage) arrives within 180s.
func TestConnect_ReceivesFollowBeforeTimeout(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 180*time.Second, func(evt events.Event) bool {
		if evt.Type != events.EventFollow {
			return false
		}
		msg, ok := evt.Data.(*pb.WebcastSocialMessage)
		if !ok || msg == nil {
			return false
		}
		nick := "?"
		if msg.User != nil {
			nick = msg.User.Nickname
		}
		t.Logf("[integration test follow] %s", nick)
		return true
	}, "no follow within 180s (follows are infrequent — try a growing stream)")
}

// TestConnect_ReceivesSubscriptionSignalBeforeTimeout — W7: disabled by default (too rare).
// Enable manually by removing the t.Skip call.
func TestConnect_ReceivesSubscriptionSignalBeforeTimeout(t *testing.T) {
	t.Skip("W7 disabled by default — subscription events are too rare on most streams; enable manually")

	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}
	awaitWssEvent(t, user, 240*time.Second, func(evt events.Event) bool {
		switch evt.Type {
		case events.EventSubNotify, events.EventSubscriptionNotify,
			events.EventSubCapsule, events.EventSubPinEvent:
			t.Logf("[integration test subscription] event type=%v", evt.Type)
			return true
		}
		return false
	}, "no subscription-related event within 240s (need subs on a sub-enabled stream)")
}

// TestDisconnect_UnblocksConnectGoroutineAfterConnected — D1: calling cancel() unblocks the
// goroutine draining the event channel within 18s of the cancel call.
func TestDisconnect_UnblocksConnectGoroutineAfterConnected(t *testing.T) {
	user := os.Getenv("PIRATETOK_LIVE_TEST_USER")
	if user == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USER to a currently-live TikTok username to run this test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := wssTestClient(user)

	eventCh, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect(%q) failed immediately: %v", user, err)
	}

	// wait for CONNECTED event (up to 90s)
	connected := make(chan struct{}, 1)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for evt := range eventCh {
			if evt.Type == events.EventConnected {
				select {
				case connected <- struct{}{}:
				default:
				}
			}
		}
	}()

	select {
	case <-connected:
		// reached CONNECTED — proceed to disconnect
	case <-time.After(90 * time.Second):
		cancel()
		<-workerDone
		t.Fatal("never reached CONNECTED within 90s (offline user or network issue)")
	}

	// cancel the context — this is the disconnect signal
	t0 := time.Now()
	cancel()

	select {
	case <-workerDone:
		elapsed := time.Since(t0)
		if elapsed > 18*time.Second {
			t.Errorf("goroutine took %v to exit after cancel — expected < 18s", elapsed)
		}
		t.Logf("[integration test disconnect] goroutine exited %.1fs after cancel", elapsed.Seconds())
	case <-time.After(20 * time.Second):
		t.Error("drain goroutine did not exit within 20s after context cancel")
	}
}
