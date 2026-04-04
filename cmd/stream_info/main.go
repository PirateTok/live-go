package main

import (
	"fmt"
	"os"
	"time"

	golive "github.com/PirateTok/live-go"
	tthttp "github.com/PirateTok/live-go/http"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: stream_info <username> [cookies]")
		os.Exit(1)
	}
	username := os.Args[1]
	cookies := ""
	if len(os.Args) > 2 {
		cookies = os.Args[2]
	}

	timeout := 10 * time.Second

	room, err := golive.CheckOnline(username, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Online check failed: %s\n", err)
		os.Exit(1)
	}

	info, err := golive.FetchRoomInfo(room.RoomID, timeout, cookies)
	if err != nil {
		switch err.(type) {
		case *tthttp.AgeRestrictedError:
			fmt.Println("Room info failed: age-restricted stream: 18+ room — pass session cookies")
			fmt.Println("Hint: if this is an 18+ room, pass session cookies as the second argument")
		default:
			fmt.Fprintf(os.Stderr, "Room info failed: %s\n", err)
		}
		os.Exit(1)
	}

	fmt.Println("=== Room Info ===")
	fmt.Printf("Username: @%s\n", username)
	fmt.Printf("Room ID:  %s\n", room.RoomID)
	fmt.Printf("Title:    %s\n", info.Title)
	fmt.Printf("Viewers:  %d\n", info.Viewers)
	fmt.Printf("Likes:    %d\n", info.Likes)
	fmt.Printf("Total:    %d unique viewers\n", info.TotalUser)

	if info.StreamURL != nil {
		fmt.Println("\n=== Stream URLs (FLV) ===")
		if info.StreamURL.FlvOrigin != "" {
			fmt.Printf("Origin: %s\n", info.StreamURL.FlvOrigin)
		}
		if info.StreamURL.FlvHD != "" {
			fmt.Printf("HD:     %s\n", info.StreamURL.FlvHD)
		}
		if info.StreamURL.FlvSD != "" {
			fmt.Printf("SD:     %s\n", info.StreamURL.FlvSD)
		}
		if info.StreamURL.FlvLD != "" {
			fmt.Printf("LD:     %s\n", info.StreamURL.FlvLD)
		}
		if info.StreamURL.FlvAudio != "" {
			fmt.Printf("Audio:  %s\n", info.StreamURL.FlvAudio)
		}
	}
}
