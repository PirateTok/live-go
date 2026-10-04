package golive

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/PirateTok/live-go/events"
)

// loopDeps are the side effects of the reconnect loop, injectable for tests.
type loopDeps struct {
	fresh func() *session                                // nil = ttwid fetch failed
	run   func(ctx context.Context, s *session) sessionExit // one WSS session
	sleep func(ctx context.Context, d time.Duration) bool // false = ctx done
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// supervise runs the reconnect loop, then emits Disconnected exactly once and
// closes the channel. ttwid + UA are fetched once and reused; they rotate only
// on DEVICE_BLOCKED, a ttwid failure, or a connection that died young.
func supervise(ctx context.Context, roomID string, maxRetries int, deps loopDeps, eventCh chan events.Event) {
	defer close(eventCh)
	reconnectLoop(ctx, roomID, maxRetries, deps, eventCh)
	emitLifecycle(ctx, eventCh, events.Event{Type: events.EventDisconnected, RoomID: roomID})
}

func reconnectLoop(ctx context.Context, roomID string, maxRetries int, deps loopDeps, eventCh chan events.Event) {
	budget := reconnectBudget{maxRetries: maxRetries}
	var held *session
	for ctx.Err() == nil {
		exit := exitNoTTWID
		var lived time.Duration
		if held == nil {
			held = deps.fresh()
		}
		if held != nil {
			started := time.Now()
			exit = deps.run(ctx, held)
			lived = time.Since(started)
		}
		if ctx.Err() != nil {
			return
		}

		j := judge(exit, lived)
		if j.rotate {
			held = nil
		}
		attempt, delay, giveUp := budget.record(j.end)
		if giveUp {
			log.Printf("max retries (%d) exceeded at attempt %d", maxRetries, attempt)
			return
		}
		emitLifecycle(ctx, eventCh, events.Event{
			Type:   events.EventReconnecting,
			RoomID: roomID,
			Data:   fmt.Sprintf("attempt=%d max=%d delay=%v", attempt, maxRetries, delay),
		})
		log.Printf("reconnecting in %v (attempt %d/%d)", delay, attempt, maxRetries)
		if !deps.sleep(ctx, delay) {
			return
		}
	}
}

// emitLifecycle never drops a lifecycle event while the stream is live; once
// the context is cancelled it delivers only if the buffer has room.
func emitLifecycle(ctx context.Context, eventCh chan events.Event, ev events.Event) {
	select {
	case eventCh <- ev:
		return
	case <-ctx.Done():
	}
	select {
	case eventCh <- ev:
	default:
	}
}

const (
	healthySession     = 30 * time.Second
	deviceBlockedDelay = 2 * time.Second
	maxBackoff         = 30 * time.Second
)

type sessionExit int

const (
	exitClosed sessionExit = iota
	exitDeviceBlocked
	exitErrored
	exitNoTTWID
)

type attemptEnd int

const (
	endHealthy attemptEnd = iota
	endFailed
	endBlocked
)

// judgement says how an attempt ended and whether to drop ttwid + UA.
type judgement struct {
	end    attemptEnd
	rotate bool
}

func judge(exit sessionExit, lived time.Duration) judgement {
	healthy := lived >= healthySession
	end := endFailed
	if healthy {
		end = endHealthy
	}
	switch exit {
	case exitDeviceBlocked:
		return judgement{end: endBlocked, rotate: true}
	case exitErrored:
		return judgement{end: end, rotate: !healthy}
	case exitNoTTWID:
		return judgement{end: endFailed, rotate: true}
	default:
		return judgement{end: end, rotate: false}
	}
}

// reconnectBudget counts consecutive failed attempts. A healthy session
// resets the count, so long-lived streams don't die after maxRetries
// lifetime blips.
type reconnectBudget struct {
	attempt    int
	maxRetries int
}

// record returns the attempt number, the delay before the next one, and
// whether to give up.
func (b *reconnectBudget) record(end attemptEnd) (int, time.Duration, bool) {
	if end == endHealthy {
		b.attempt = 1
	} else {
		b.attempt++
	}
	if b.attempt > b.maxRetries {
		return b.attempt, 0, true
	}
	if end == endBlocked {
		return b.attempt, deviceBlockedDelay, false
	}
	return b.attempt, backoff(b.attempt), false
}

func backoff(attempt int) time.Duration {
	if attempt >= 5 {
		return maxBackoff
	}
	return time.Duration(1<<attempt) * time.Second
}
