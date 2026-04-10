package main

import (
	"fmt"
	"os"
	"time"

	tthttp "github.com/PirateTok/live-go/http"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: online_check <username> [username2] ...")
		os.Exit(1)
	}

	timeout := 10 * time.Second

	for _, username := range os.Args[1:] {
		result, err := tthttp.CheckOnline(username, timeout, "", "", "")
		if err != nil {
			switch err.(type) {
			case *tthttp.UserNotFoundError:
				fmt.Printf("  404   @%s — user does not exist\n", username)
			case *tthttp.HostNotOnlineError:
				fmt.Printf("  OFF   @%s — not currently live\n", username)
			case *tthttp.TikTokBlockedError:
				fmt.Printf("  BLOCK @%s — %s\n", username, err)
			case *tthttp.TikTokAPIError:
				fmt.Printf("  ERR   @%s — %s\n", username, err)
			default:
				fmt.Printf("  FAIL  @%s — %s\n", username, err)
			}
			continue
		}
		fmt.Printf("  LIVE  @%s — room %s\n", username, result.RoomID)
	}
}
