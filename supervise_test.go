package golive

import (
	"context"
	"testing"
	"time"

	"github.com/PirateTok/live-go/events"
)

type fakeLoop struct {
	freshCalls int
	freshFails int // first N fresh() calls fail
	exits      []sessionExit
	runCalls   int
	sleeps     []time.Duration
}

func (f *fakeLoop) deps() loopDeps {
	return loopDeps{
		fresh: func() *session {
			f.freshCalls++
			if f.freshCalls <= f.freshFails {
				return nil
			}
			return &session{ttwid: "t", ua: "u"}
		},
		run: func(context.Context, *session) sessionExit {
			exit := f.exits[f.runCalls%len(f.exits)]
			f.runCalls++
			return exit
		},
		sleep: func(_ context.Context, d time.Duration) bool {
			f.sleeps = append(f.sleeps, d)
			return true
		},
	}
}

func drain(ch chan events.Event) []events.EventType {
	var types []events.EventType
	for ev := range ch {
		types = append(types, ev.Type)
	}
	return types
}

func count(types []events.EventType, want events.EventType) int {
	n := 0
	for _, t := range types {
		if t == want {
			n++
		}
	}
	return n
}

// F4 + F5 at client level: ttwid failures are attempts, the budget runs out,
// Reconnecting fires once per retry and Disconnected exactly once, last.
func TestSuperviseReconnectingThenDisconnectedOnce(t *testing.T) {
	f := &fakeLoop{freshFails: 2, exits: []sessionExit{exitErrored}}
	ch := make(chan events.Event, 1) // tiny buffer: lifecycle events must not be dropped
	done := make(chan []events.EventType)
	go func() { done <- drain(ch) }()

	supervise(context.Background(), "7", 3, f.deps(), ch)
	types := <-done

	want := []events.EventType{events.EventReconnecting, events.EventReconnecting, events.EventReconnecting, events.EventDisconnected}
	if len(types) != len(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events = %v, want %v", types, want)
		}
	}
	// 2 ttwid failures + 2 young-error sessions, each rotating → fresh() every attempt
	if f.freshCalls != 4 || f.runCalls != 2 {
		t.Fatalf("fresh=%d run=%d", f.freshCalls, f.runCalls)
	}
	if len(f.sleeps) != 3 || f.sleeps[0] != 2*time.Second || f.sleeps[2] != 8*time.Second {
		t.Fatalf("backoff = %v", f.sleeps)
	}
}

func TestSuperviseReusesSessionOnCleanClose(t *testing.T) {
	f := &fakeLoop{exits: []sessionExit{exitClosed}}
	ch := make(chan events.Event, 64)
	supervise(context.Background(), "7", 4, f.deps(), ch)
	types := drain(ch)
	if f.freshCalls != 1 || f.runCalls != 5 {
		t.Fatalf("fresh=%d run=%d: clean close must keep ttwid + UA", f.freshCalls, f.runCalls)
	}
	if count(types, events.EventReconnecting) != 4 || count(types, events.EventDisconnected) != 1 {
		t.Fatalf("events = %v", types)
	}
}

func TestSuperviseDeviceBlockedRotatesWithShortDelay(t *testing.T) {
	f := &fakeLoop{exits: []sessionExit{exitDeviceBlocked}}
	ch := make(chan events.Event, 64)
	supervise(context.Background(), "7", 2, f.deps(), ch)
	drain(ch)
	if f.freshCalls != 3 || f.sleeps[0] != deviceBlockedDelay {
		t.Fatalf("fresh=%d sleeps=%v", f.freshCalls, f.sleeps)
	}
}

func TestSuperviseUserCancelDisconnectsOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	deps := loopDeps{
		fresh: func() *session { return &session{} },
		run: func(ctx context.Context, _ *session) sessionExit {
			cancel()
			<-ctx.Done()
			return exitClosed
		},
		sleep: sleepCtx,
	}
	ch := make(chan events.Event, 8)
	supervise(ctx, "7", 5, deps, ch)
	types := drain(ch)
	if len(types) != 1 || types[0] != events.EventDisconnected {
		t.Fatalf("events = %v, want exactly [Disconnected]", types)
	}
}
