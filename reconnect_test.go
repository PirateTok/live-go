package golive

import (
	"testing"
	"time"
)

const (
	short = 3 * time.Second
	long  = 45 * time.Second
)

func TestBudgetFailuresAccumulateUntilGiveUp(t *testing.T) {
	b := reconnectBudget{maxRetries: 3}
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	for i, d := range want {
		attempt, delay, giveUp := b.record(endFailed)
		if giveUp || attempt != i+1 || delay != d {
			t.Fatalf("step %d: attempt=%d delay=%v giveUp=%v", i, attempt, delay, giveUp)
		}
	}
	if attempt, _, giveUp := b.record(endFailed); !giveUp || attempt != 4 {
		t.Fatalf("expected give up at 4, got attempt=%d giveUp=%v", attempt, giveUp)
	}
}

func TestBudgetHealthyResets(t *testing.T) {
	b := reconnectBudget{maxRetries: 3}
	b.record(endFailed)
	b.record(endFailed)
	b.record(endFailed)
	if attempt, _, giveUp := b.record(endHealthy); giveUp || attempt != 1 {
		t.Fatalf("healthy: attempt=%d giveUp=%v", attempt, giveUp)
	}
	for i := 0; i < 20; i++ {
		if _, _, giveUp := b.record(endFailed); giveUp {
			t.Fatalf("blip %d exhausted budget despite healthy sessions", i)
		}
		if _, _, giveUp := b.record(endHealthy); giveUp {
			t.Fatalf("healthy %d gave up", i)
		}
	}
}

func TestBudgetBlockedShortDelayStillCounts(t *testing.T) {
	b := reconnectBudget{maxRetries: 2}
	if attempt, delay, _ := b.record(endBlocked); attempt != 1 || delay != deviceBlockedDelay {
		t.Fatalf("attempt=%d delay=%v", attempt, delay)
	}
	b.record(endBlocked)
	if _, _, giveUp := b.record(endBlocked); !giveUp {
		t.Fatal("blocked attempts must count toward the budget")
	}
}

func TestBackoffCaps(t *testing.T) {
	if backoff(4) != 16*time.Second || backoff(5) != maxBackoff || backoff(1<<30) != maxBackoff {
		t.Fatalf("backoff(4)=%v backoff(5)=%v", backoff(4), backoff(5))
	}
}

func TestJudgeRotation(t *testing.T) {
	cases := []struct {
		name   string
		exit   sessionExit
		lived  time.Duration
		end    attemptEnd
		rotate bool
	}{
		{"device blocked rotates", exitDeviceBlocked, long, endBlocked, true},
		{"ttwid failure is a failed attempt, refetch", exitNoTTWID, 0, endFailed, true},
		{"error that died young rotates", exitErrored, short, endFailed, true},
		{"error after healthy session keeps", exitErrored, long, endHealthy, false},
		{"short clean close keeps", exitClosed, short, endFailed, false},
		{"long clean close keeps", exitClosed, long, endHealthy, false},
	}
	for _, c := range cases {
		j := judge(c.exit, c.lived)
		if j.end != c.end || j.rotate != c.rotate {
			t.Errorf("%s: end=%v rotate=%v, want end=%v rotate=%v", c.name, j.end, j.rotate, c.end, c.rotate)
		}
	}
}
