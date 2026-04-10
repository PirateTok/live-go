package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	golive "github.com/PirateTok/live-go"
	"github.com/PirateTok/live-go/events"
	"github.com/PirateTok/live-go/helpers"
	pb "github.com/PirateTok/live-go/proto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gift_streak <username>")
		os.Exit(1)
	}
	username := os.Args[1]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("Connecting to @%s...\n", username)
	client := golive.NewClient(username)
	eventCh, err := client.Connect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %s\n", err)
		os.Exit(1)
	}

	tracker := helpers.NewGiftStreakTracker()

	for evt := range eventCh {
		switch evt.Type {
		case events.EventConnected:
			fmt.Printf("Connected to room %s! Tracking gift streaks...\n\n", evt.RoomID)

		case events.EventGift:
			msg := evt.Data.(*pb.WebcastGiftMessage)
			streak := tracker.Process(msg)

			nick := "?"
			if msg.User != nil {
				nick = msg.User.Nickname
			}

			giftName := fmt.Sprintf("gift#%d", msg.GiftId)
			if msg.Gift != nil && msg.Gift.Name != "" {
				giftName = msg.Gift.Name
			}

			status := "active"
			if streak.IsFinal {
				status = "final"
			}

			fmt.Printf("[%s] %s sent %s  streak_id=%d  event_gifts=%d  event_diamonds=%d  total_gifts=%d  total_diamonds=%d\n",
				status, nick, giftName,
				streak.StreakID, streak.EventGiftCount, streak.EventDiamondCount,
				streak.TotalGiftCount, streak.TotalDiamondCount)

		case events.EventLiveEnded:
			fmt.Println("\n[stream ended]")
			return

		case events.EventDisconnected:
			fmt.Println("\n[disconnected]")
			return
		}
	}
}
