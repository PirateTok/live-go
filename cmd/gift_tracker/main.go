package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"

	golive "github.com/PirateTok/live-go"
	"github.com/PirateTok/live-go/events"
	pb "github.com/PirateTok/live-go/proto"
)

type giftEntry struct {
	Name     string
	Diamonds int64
	Count    int32
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gift_tracker <username>")
		os.Exit(1)
	}
	username := os.Args[1]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("Connecting to @%s...\n", username)
	client := golive.NewClient(username)
	eventCh, err := client.Connect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", err)
		os.Exit(1)
	}

	var mu sync.Mutex
	totals := make(map[string]*giftEntry)
	var totalDiamonds int64

	for evt := range eventCh {
		switch evt.Type {
		case events.EventConnected:
			fmt.Printf("Connected to room %s! Tracking gifts...\n\n", evt.RoomID)

		case events.EventGift:
			msg := evt.Data.(*pb.WebcastGiftMessage)

			nick := "?"
			if msg.User != nil {
				nick = msg.User.Nickname
			}

			giftName := fmt.Sprintf("gift#%d", msg.GiftId)
			var diamonds int64
			if msg.Gift != nil {
				if msg.Gift.Describe != "" {
					giftName = msg.Gift.Describe
				}
				diamonds = int64(msg.Gift.DiamondCount)
			}

			repeatEnd := msg.RepeatEnd == 1

			mu.Lock()
			key := fmt.Sprintf("%s:%d", nick, msg.GiftId)
			entry, ok := totals[key]
			if !ok {
				entry = &giftEntry{Name: giftName, Diamonds: diamonds}
				totals[key] = entry
			}
			entry.Count = msg.RepeatCount
			if repeatEnd {
				totalDiamonds += diamonds * int64(msg.RepeatCount)
			}
			mu.Unlock()

			if repeatEnd {
				fmt.Printf("[gift] %s sent %dx %s (%d diamonds each)\n",
					nick, msg.RepeatCount, giftName, diamonds)
			}

		case events.EventLiveEnded:
			fmt.Println("\n[stream ended]")
			cancel()

		case events.EventDisconnected:
			fmt.Println("\n[disconnected]")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(totals) > 0 {
		printSummary(totals, totalDiamonds)
	}
}

func printSummary(totals map[string]*giftEntry, totalDiamonds int64) {
	type row struct {
		Key   string
		Entry *giftEntry
		Total int64
	}
	var rows []row
	for k, e := range totals {
		rows = append(rows, row{Key: k, Entry: e, Total: e.Diamonds * int64(e.Count)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Total > rows[j].Total })

	fmt.Printf("\n=== Gift Summary ===\n")
	for _, r := range rows {
		fmt.Printf("  %s: %dx %s = %d diamonds\n", r.Key, r.Entry.Count, r.Entry.Name, r.Total)
	}
	fmt.Printf("\nTotal diamonds: %d\n", totalDiamonds)
}
