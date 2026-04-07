package main

import (
	"fmt"
	"os"

	tthttp "github.com/PirateTok/live-go/http"
)

func main() {
	username := "tiktok"
	if len(os.Args) > 1 {
		username = os.Args[1]
	}

	cache := tthttp.NewProfileCache()

	fmt.Printf("Fetching profile for @%s...\n", username)
	p, err := cache.Fetch(username)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	fmt.Printf("  User ID:    %s\n", p.UserID)
	fmt.Printf("  Nickname:   %s\n", p.Nickname)
	fmt.Printf("  Verified:   %v\n", p.Verified)
	fmt.Printf("  Followers:  %d\n", p.FollowerCount)
	fmt.Printf("  Videos:     %d\n", p.VideoCount)
	fmt.Printf("  Avatar (thumb):  %s\n", p.AvatarThumb)
	fmt.Printf("  Avatar (720):    %s\n", p.AvatarMedium)
	fmt.Printf("  Avatar (1080):   %s\n", p.AvatarLarge)
	bioLink := "(none)"
	if p.BioLink != "" {
		bioLink = p.BioLink
	}
	fmt.Printf("  Bio link:   %s\n", bioLink)
	roomID := "(offline)"
	if p.RoomID != "" {
		roomID = p.RoomID
	}
	fmt.Printf("  Room ID:    %s\n", roomID)

	fmt.Printf("\nFetching @%s again (should be cached)...\n", username)
	p2, err := cache.Fetch(username)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}
	fmt.Printf("  [cached] %s — %d followers\n", p2.Nickname, p2.FollowerCount)
}
