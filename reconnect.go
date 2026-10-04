package golive

import "time"

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
