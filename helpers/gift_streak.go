package helpers

import (
	"time"

	"github.com/PirateTok/live-go/proto"
)

const staleSecs = 60

// GiftStreakEvent contains enriched gift data with per-event deltas.
type GiftStreakEvent struct {
	StreakID          int64
	IsActive          bool
	IsFinal           bool
	EventGiftCount    int32
	TotalGiftCount    int32
	EventDiamondCount int64
	TotalDiamondCount int64
}

type streakEntry struct {
	lastRepeatCount int32
	lastSeen        time.Time
}

// GiftStreakTracker tracks gift streak deltas from TikTok's running totals.
// Not safe for concurrent use.
type GiftStreakTracker struct {
	streaks map[int64]*streakEntry
}

// NewGiftStreakTracker creates a new tracker.
func NewGiftStreakTracker() *GiftStreakTracker {
	return &GiftStreakTracker{streaks: make(map[int64]*streakEntry)}
}

// Process takes a raw gift message and returns enriched streak data with deltas.
func (t *GiftStreakTracker) Process(msg *proto.WebcastGiftMessage) GiftStreakEvent {
	var diamondPer int32
	if msg.Gift != nil {
		diamondPer = msg.Gift.DiamondCount
	}

	isCombo := msg.Gift != nil && msg.Gift.Type == 1
	isFinal := msg.RepeatEnd == 1

	if !isCombo {
		return GiftStreakEvent{
			StreakID:          msg.GroupId,
			IsActive:          false,
			IsFinal:           true,
			EventGiftCount:    1,
			TotalGiftCount:    1,
			EventDiamondCount: int64(diamondPer),
			TotalDiamondCount: int64(diamondPer),
		}
	}

	now := time.Now()
	t.evictStale(now)

	var prevCount int32
	if prev, ok := t.streaks[msg.GroupId]; ok {
		prevCount = prev.lastRepeatCount
	}

	delta := msg.RepeatCount - prevCount
	if delta < 0 {
		delta = 0
	}

	if isFinal {
		delete(t.streaks, msg.GroupId)
	} else {
		t.streaks[msg.GroupId] = &streakEntry{
			lastRepeatCount: msg.RepeatCount,
			lastSeen:        now,
		}
	}

	rc := int64(msg.RepeatCount)
	if rc < 1 {
		rc = 1
	}

	return GiftStreakEvent{
		StreakID:          msg.GroupId,
		IsActive:          !isFinal,
		IsFinal:           isFinal,
		EventGiftCount:    delta,
		TotalGiftCount:    msg.RepeatCount,
		EventDiamondCount: int64(diamondPer) * int64(delta),
		TotalDiamondCount: int64(diamondPer) * rc,
	}
}

// ActiveStreaks returns the number of currently active (non-finalized) streaks.
func (t *GiftStreakTracker) ActiveStreaks() int {
	return len(t.streaks)
}

// Reset clears all tracked state. For reconnect scenarios.
func (t *GiftStreakTracker) Reset() {
	t.streaks = make(map[int64]*streakEntry)
}

func (t *GiftStreakTracker) evictStale(now time.Time) {
	for id, entry := range t.streaks {
		if now.Sub(entry.lastSeen).Seconds() >= staleSecs {
			delete(t.streaks, id)
		}
	}
}
