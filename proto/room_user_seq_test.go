package proto

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func ranked(rank, score, userID int64, nick string) *WebcastRoomUserSeqMessage_Contributor {
	return &WebcastRoomUserSeqMessage_Contributor{Rank: rank, Score: score, User: &User{Id: userID, Nickname: nick}}
}

func TestRoomUserSeqDecodeAndTopViewers(t *testing.T) {
	wire, err := proto.Marshal(&WebcastRoomUserSeqMessage{
		RanksList: []*WebcastRoomUserSeqMessage_Contributor{
			ranked(3, 10, 300, "third"),
			{Rank: 0, Score: 999}, // no user — skipped
			ranked(1, 5000, 100, "first"),
			ranked(2, 1200, 200, "second"),
		},
		ViewerCount: 321,
		TotalUser:   4567,
		Anonymous:   12,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var msg WebcastRoomUserSeqMessage
	if err := proto.Unmarshal(wire, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(msg.RanksList) != 4 || msg.ViewerCount != 321 || msg.TotalUser != 4567 || msg.Anonymous != 12 {
		t.Fatalf("decoded = %+v", &msg)
	}
	top := msg.TopViewers()
	if len(top) != 3 {
		t.Fatalf("top viewers = %d, want 3", len(top))
	}
	for i, want := range []string{"first", "second", "third"} {
		if top[i].GetRank() != int64(i+1) || top[i].GetUser().GetNickname() != want {
			t.Fatalf("top[%d] = rank %d %q", i, top[i].GetRank(), top[i].GetUser().GetNickname())
		}
	}
	if top[0].GetScore() != 5000 || top[0].GetUser().GetId() != 100 {
		t.Fatalf("top[0] = %+v", top[0])
	}
}

func TestTopViewersEmpty(t *testing.T) {
	if len((&WebcastRoomUserSeqMessage{}).TopViewers()) != 0 {
		t.Fatal("expected no top viewers")
	}
}
