package helpers

import (
	"github.com/PirateTok/live-go/proto"
)

// LikeStats contains monotonized like statistics.
type LikeStats struct {
	EventLikeCount  int32
	TotalLikeCount  int32
	AccumulatedCount int64
	WentBackwards   bool
}

// LikeAccumulator monotonizes TikTok's inconsistent total_like_count.
// Not safe for concurrent use.
type LikeAccumulator struct {
	maxTotal    int32
	accumulated int64
}

// NewLikeAccumulator creates a new accumulator.
func NewLikeAccumulator() *LikeAccumulator {
	return &LikeAccumulator{}
}

// Process takes a raw like message and returns monotonized stats.
func (a *LikeAccumulator) Process(msg *proto.WebcastLikeMessage) LikeStats {
	a.accumulated += int64(msg.Count)
	wentBackwards := msg.Total < a.maxTotal
	if msg.Total > a.maxTotal {
		a.maxTotal = msg.Total
	}

	return LikeStats{
		EventLikeCount:  msg.Count,
		TotalLikeCount:  a.maxTotal,
		AccumulatedCount: a.accumulated,
		WentBackwards:   wentBackwards,
	}
}

// Reset clears state. For reconnect.
func (a *LikeAccumulator) Reset() {
	a.maxTotal = 0
	a.accumulated = 0
}
