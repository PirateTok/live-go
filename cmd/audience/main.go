// audience fetches the full viewer roster of a live room, then exits.
//
// TikTok gates this endpoint behind a login, so session cookies are required:
//
//	go run ./cmd/audience <username> "sessionid=abc; sid_tt=abc"
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	golive "github.com/PirateTok/live-go"
	tthttp "github.com/PirateTok/live-go/http"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println(`Usage: audience <username> "sessionid=xxx; sid_tt=xxx"`)
		os.Exit(1)
	}
	username, cookies := os.Args[1], os.Args[2]
	timeout := 10 * time.Second

	room, err := golive.CheckOnline(username, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Online check failed: %s\n", err)
		os.Exit(1)
	}

	audience, err := golive.FetchRoomAudience(room.RoomID, room.AnchorID, timeout, cookies)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audience failed: %s\n", err)
		var sr *tthttp.SessionRequiredError
		if errors.As(err, &sr) {
			fmt.Fprintln(os.Stderr, "Hint: copy sessionid + sid_tt from browser DevTools while logged in")
		}
		os.Exit(1)
	}

	fmt.Printf("@%s — %d in room (%d anonymous), %d listed\n", username, audience.Total, audience.Anonymous, len(audience.Viewers))
	for _, v := range audience.Viewers {
		tags := ""
		if v.IsSubscriber {
			tags += " [sub]"
		}
		if v.IsFollower {
			tags += " [follower]"
		}
		fmt.Printf("#%-3d @%s (%s) score=%d followers=%d%s\n", v.Rank, v.Username, v.Nickname, v.Score, v.FollowerCount, tags)
	}
}
