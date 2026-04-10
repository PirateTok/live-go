//go:build integration

package golive

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PirateTok/live-go/events"
)

// TestMultipleClients_TrackChatForOneMinute — M1: connect N clients concurrently,
// count chat events for 60s, then disconnect all cleanly.
//
// Gate: PIRATETOK_LIVE_TEST_USERS (comma-separated, all must be live).
func TestMultipleClients_TrackChatForOneMinute(t *testing.T) {
	raw := os.Getenv("PIRATETOK_LIVE_TEST_USERS")
	if raw == "" {
		t.Skip("set PIRATETOK_LIVE_TEST_USERS to a comma-separated list of live TikTok usernames to run this test")
	}

	parts := strings.Split(raw, ",")
	usernames := make([]string, 0, len(parts))
	for _, p := range parts {
		if u := strings.TrimSpace(p); u != "" {
			usernames = append(usernames, u)
		}
	}
	if len(usernames) == 0 {
		t.Skip("PIRATETOK_LIVE_TEST_USERS contained no non-empty usernames")
	}
	t.Logf("[integration test multi-stream] connecting %d clients: %v", len(usernames), usernames)

	type session struct {
		username  string
		cancel    context.CancelFunc
		eventCh   <-chan events.Event
		chatCount atomic.Int64
	}

	sessions := make([]*session, len(usernames))
	for i, u := range usernames {
		ctx, cancel := context.WithCancel(context.Background())
		client := NewClient(u).
			CdnEU().
			Timeout(15 * time.Second).
			MaxRetries(5).
			StaleTimeout(120 * time.Second)
		ch, err := client.Connect(ctx)
		if err != nil {
			cancel()
			t.Fatalf("Connect(%q) failed immediately: %v", u, err)
		}
		sessions[i] = &session{username: u, cancel: cancel, eventCh: ch}
	}

	// wait for all clients to reach CONNECTED
	connected := make(chan string, len(sessions))
	var connectWg sync.WaitGroup
	for _, s := range sessions {
		connectWg.Add(1)
		go func(s *session) {
			defer connectWg.Done()
			for evt := range s.eventCh {
				if evt.Type == events.EventConnected {
					connected <- s.username
					return
				}
				if evt.Type == events.EventDisconnected {
					return
				}
			}
		}(s)
	}

	allConnected := make(chan struct{})
	go func() {
		connectWg.Wait()
		close(allConnected)
	}()

	timer := time.NewTimer(120 * time.Second)
	defer timer.Stop()
	connectedCount := 0
	waiting := true
	for waiting {
		select {
		case u := <-connected:
			connectedCount++
			t.Logf("[integration test multi-stream] connected: @%s (%d/%d)", u, connectedCount, len(sessions))
			if connectedCount == len(sessions) {
				waiting = false
			}
		case <-allConnected:
			waiting = false
		case <-timer.C:
			for _, s := range sessions {
				s.cancel()
			}
			t.Fatalf("not all clients reached CONNECTED within 120s (%d/%d connected)", connectedCount, len(sessions))
		}
	}

	t.Logf("[integration test multi-stream] all %d clients connected — tracking chat for 60s", len(sessions))

	// drain all channels, count chat events for 60s
	liveWindow := time.NewTimer(60 * time.Second)
	defer liveWindow.Stop()

	var drainWg sync.WaitGroup
	for _, s := range sessions {
		drainWg.Add(1)
		go func(s *session) {
			defer drainWg.Done()
			for evt := range s.eventCh {
				if evt.Type == events.EventChat {
					s.chatCount.Add(1)
				}
			}
		}(s)
	}

	// wait for live window to expire, then cancel all
	<-liveWindow.C
	t.Log("[integration test multi-stream] 60s elapsed — disconnecting all clients")
	for _, s := range sessions {
		s.cancel()
	}

	// wait for all drain goroutines to finish (channels close after cancel)
	drainDone := make(chan struct{})
	go func() {
		drainWg.Wait()
		close(drainDone)
	}()

	select {
	case <-drainDone:
		// all goroutines exited cleanly
	case <-time.After(120 * time.Second):
		t.Error("drain goroutines did not exit within 120s after disconnect")
	}

	// log per-channel results
	for _, s := range sessions {
		t.Logf("[integration test multi-stream] @%s: %d chat events in 60s", s.username, s.chatCount.Load())
	}
}
