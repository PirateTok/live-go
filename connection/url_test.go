package connection

import (
	"strings"
	"testing"
	"time"
)

func TestBuildWSSURLHeartbeatDuration(t *testing.T) {
	if u := BuildWSSURL("h", "1", "UTC", "en", "US", true, 7*time.Second); !strings.Contains(u, "heartbeat_duration=7000&") {
		t.Fatalf("custom interval not in URL: %s", u)
	}
	if u := BuildWSSURL("h", "1", "UTC", "en", "US", true, 0); !strings.Contains(u, "heartbeat_duration=10000&") {
		t.Fatalf("default interval not in URL: %s", u)
	}
}
