package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	golive "github.com/PirateTok/live-go"
	"github.com/PirateTok/live-go/events"
	pb "github.com/PirateTok/live-go/proto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: basic_chat <username>")
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

	for evt := range eventCh {
		switch evt.Type {
		case events.EventConnected:
			fmt.Printf("Connected to room %s! Waiting for chat messages...\n\n", evt.RoomID)

		case events.EventChat:
			msg := evt.Data.(*pb.WebcastChatMessage)
			nick := "?"
			if msg.User != nil {
				nick = msg.User.Nickname
			}
			fmt.Printf("%s: %s\n", nick, msg.Content)

		case events.EventFollow:
			msg := evt.Data.(*pb.WebcastSocialMessage)
			nick := "?"
			if msg.User != nil {
				nick = msg.User.Nickname
			}
			fmt.Printf("[follow] %s\n", nick)

		case events.EventShare:
			msg := evt.Data.(*pb.WebcastSocialMessage)
			nick := "?"
			if msg.User != nil {
				nick = msg.User.Nickname
			}
			fmt.Printf("[share] %s\n", nick)

		case events.EventJoin:
			msg := evt.Data.(*pb.WebcastMemberMessage)
			fmt.Printf("[join] member_count=%d\n", msg.MemberCount)

		case events.EventLike:
			msg := evt.Data.(*pb.WebcastLikeMessage)
			nick := "?"
			if msg.User != nil {
				nick = msg.User.Nickname
			}
			fmt.Printf("[like] %s (%d total)\n", nick, msg.Total)

		case events.EventRoomUserSeq:
			msg := evt.Data.(*pb.WebcastRoomUserSeqMessage)
			fmt.Printf("[viewers] %d total, pop=%d\n", msg.TotalUser, msg.Popularity)

		case events.EventLiveEnded:
			fmt.Println("[control] stream ended")
			return

		case events.EventDisconnected:
			fmt.Println("[disconnected]")
			return

		// skip raw Social/Member/Control — handled via convenience events
		case events.EventSocial, events.EventMember, events.EventControl:
		}
	}
}
