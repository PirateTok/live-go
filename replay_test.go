package golive

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/PirateTok/live-go/events"
	"github.com/PirateTok/live-go/helpers"
	pb "github.com/PirateTok/live-go/proto"
	"google.golang.org/protobuf/proto"
)

// --- manifest JSON types ---

type manifest struct {
	FrameCount          uint64                       `json:"frame_count"`
	MessageCount        uint64                       `json:"message_count"`
	EventCount          uint64                       `json:"event_count"`
	DecodeFailures      uint64                       `json:"decode_failures"`
	DecompressFailures  uint64                       `json:"decompress_failures"`
	PayloadTypes        map[string]uint64             `json:"payload_types"`
	MessageTypes        map[string]uint64             `json:"message_types"`
	EventTypes          map[string]uint64             `json:"event_types"`
	SubRouted           subRouted                    `json:"sub_routed"`
	UnknownTypes        map[string]uint64             `json:"unknown_types"`
	LikeAccumulator     likeManifest                 `json:"like_accumulator"`
	GiftStreaks         giftManifest                 `json:"gift_streaks"`
}

type subRouted struct {
	Follow    uint64 `json:"follow"`
	Share     uint64 `json:"share"`
	Join      uint64 `json:"join"`
	LiveEnded uint64 `json:"live_ended"`
}

type likeManifest struct {
	EventCount         uint64      `json:"event_count"`
	BackwardsJumps     uint64      `json:"backwards_jumps"`
	FinalMaxTotal      int32       `json:"final_max_total"`
	FinalAccumulated   int64       `json:"final_accumulated"`
	AccTotalMonotonic  bool        `json:"acc_total_monotonic"`
	AccumulatedMono    bool        `json:"accumulated_monotonic"`
	Events             []likeEvent `json:"events"`
}

type likeEvent struct {
	WireCount     int32 `json:"wire_count"`
	WireTotal     int32 `json:"wire_total"`
	AccTotal      int32 `json:"acc_total"`
	Accumulated   int64 `json:"accumulated"`
	WentBackwards bool  `json:"went_backwards"`
}

type giftManifest struct {
	EventCount    uint64                          `json:"event_count"`
	ComboCount    uint64                          `json:"combo_count"`
	NonComboCount uint64                          `json:"non_combo_count"`
	StreakFinals   uint64                          `json:"streak_finals"`
	NegativeDeltas uint64                         `json:"negative_deltas"`
	Groups        map[string][]giftGroupEvent     `json:"groups"`
}

type giftGroupEvent struct {
	GiftID       int32 `json:"gift_id"`
	RepeatCount  int32 `json:"repeat_count"`
	Delta        int32 `json:"delta"`
	IsFinal      bool  `json:"is_final"`
	DiamondTotal int64 `json:"diamond_total"`
}

// --- replay result ---

type replayResult struct {
	frameCount         uint64
	messageCount       uint64
	eventCount         uint64
	decodeFailures     uint64
	decompressFailures uint64
	payloadTypes       map[string]uint64
	messageTypes       map[string]uint64
	eventTypes         map[string]uint64
	followCount        uint64
	shareCount         uint64
	joinCount          uint64
	liveEndedCount     uint64
	unknownTypes       map[string]uint64
	likeEvents         []likeEventResult
	giftGroups         map[string][]giftGroupResult
	comboCount         uint64
	nonComboCount      uint64
	streakFinals       uint64
	negativeDeltas     uint64
}

type likeEventResult struct {
	wireCount     int32
	wireTotal     int32
	accTotal      int32
	accumulated   int64
	wentBackwards bool
}

type giftGroupResult struct {
	giftID       int32
	repeatCount  int32
	delta        int32
	isFinal      bool
	diamondTotal int64
}

// --- testdata location ---

func findCapturePath(name string) (capPath, manPath string, found bool) {
	// 1. $PIRATETOK_TESTDATA
	if dir := os.Getenv("PIRATETOK_TESTDATA"); dir != "" {
		cap := filepath.Join(dir, "captures", name+".bin")
		man := filepath.Join(dir, "manifests", name+".json")
		if fileExists(cap) && fileExists(man) {
			return cap, man, true
		}
	}

	// 2. testdata/ in repo root
	cap := filepath.Join("testdata", "captures", name+".bin")
	man := filepath.Join("testdata", "manifests", name+".json")
	if fileExists(cap) && fileExists(man) {
		return cap, man, true
	}

	return "", "", false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// --- frame reader ---

func readCapture(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read capture %s: %v", path, err)
	}

	var frames [][]byte
	pos := 0
	for pos+4 <= len(data) {
		frameLen := binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
		end := pos + int(frameLen)
		if end > len(data) {
			t.Fatalf("truncated frame at offset %d", pos-4)
		}
		frame := make([]byte, frameLen)
		copy(frame, data[pos:end])
		frames = append(frames, frame)
		pos = end
	}
	return frames
}

// --- gzip decompression (same logic as connection.DecompressIfGzipped) ---

func decompressIfGzipped(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	return out, nil
}

// --- event type name mapping (must match manifest canonical names) ---

func eventTypeName(evt events.Event) string {
	switch evt.Type {
	case events.EventConnected:
		return "Connected"
	case events.EventReconnecting:
		return "Reconnecting"
	case events.EventDisconnected:
		return "Disconnected"
	case events.EventChat:
		return "Chat"
	case events.EventGift:
		return "Gift"
	case events.EventLike:
		return "Like"
	case events.EventMember:
		return "Member"
	case events.EventSocial:
		return "Social"
	case events.EventFollow:
		return "Follow"
	case events.EventShare:
		return "Share"
	case events.EventJoin:
		return "Join"
	case events.EventRoomUserSeq:
		return "RoomUserSeq"
	case events.EventControl:
		return "Control"
	case events.EventLiveEnded:
		return "LiveEnded"
	case events.EventLiveIntro:
		return "LiveIntro"
	case events.EventRoomMessage:
		return "RoomMessage"
	case events.EventCaption:
		return "Caption"
	case events.EventGoalUpdate:
		return "GoalUpdate"
	case events.EventImDelete:
		return "ImDelete"
	case events.EventRankUpdate:
		return "RankUpdate"
	case events.EventPoll:
		return "Poll"
	case events.EventEnvelope:
		return "Envelope"
	case events.EventRoomPin:
		return "RoomPin"
	case events.EventUnauthorizedMember:
		return "UnauthorizedMember"
	case events.EventLinkMicMethod:
		return "LinkMicMethod"
	case events.EventLinkMicBattle:
		return "LinkMicBattle"
	case events.EventLinkMicArmies:
		return "LinkMicArmies"
	case events.EventLinkMessage:
		return "LinkMessage"
	case events.EventLinkLayer:
		return "LinkLayer"
	case events.EventLinkMicLayoutState:
		return "LinkMicLayoutState"
	case events.EventGiftPanelUpdate:
		return "GiftPanelUpdate"
	case events.EventInRoomBanner:
		return "InRoomBanner"
	case events.EventGuide:
		return "Guide"
	case events.EventEmoteChat:
		return "EmoteChat"
	case events.EventQuestionNew:
		return "QuestionNew"
	case events.EventSubNotify:
		return "SubNotify"
	case events.EventBarrage:
		return "Barrage"
	case events.EventHourlyRank:
		return "HourlyRank"
	case events.EventMsgDetect:
		return "MsgDetect"
	case events.EventLinkMicFanTicket:
		return "LinkMicFanTicket"
	case events.EventRoomVerify:
		return "RoomVerify"
	case events.EventOecLiveShopping:
		return "OecLiveShopping"
	case events.EventGiftBroadcast:
		return "GiftBroadcast"
	case events.EventRankText:
		return "RankText"
	case events.EventGiftDynamicRestriction:
		return "GiftDynamicRestriction"
	case events.EventViewerPicksUpdate:
		return "ViewerPicksUpdate"
	case events.EventSystemMessage:
		return "SystemMessage"
	case events.EventLiveGameIntro:
		return "LiveGameIntro"
	case events.EventAccessControl:
		return "AccessControl"
	case events.EventAccessRecall:
		return "AccessRecall"
	case events.EventAlertBoxAuditResult:
		return "AlertBoxAuditResult"
	case events.EventBindingGift:
		return "BindingGift"
	case events.EventBoostCard:
		return "BoostCard"
	case events.EventBottomMessage:
		return "BottomMessage"
	case events.EventGameRankNotify:
		return "GameRankNotify"
	case events.EventGiftPrompt:
		return "GiftPrompt"
	case events.EventLinkState:
		return "LinkState"
	case events.EventLinkMicBattlePunishFinish:
		return "LinkMicBattlePunishFinish"
	case events.EventLinkmicBattleTask:
		return "LinkmicBattleTask"
	case events.EventMarqueeAnnouncement:
		return "MarqueeAnnouncement"
	case events.EventNotice:
		return "Notice"
	case events.EventNotify:
		return "Notify"
	case events.EventPartnershipDropsUpdate:
		return "PartnershipDropsUpdate"
	case events.EventPartnershipGameOffline:
		return "PartnershipGameOffline"
	case events.EventPartnershipPunish:
		return "PartnershipPunish"
	case events.EventPerception:
		return "Perception"
	case events.EventSpeaker:
		return "Speaker"
	case events.EventSubCapsule:
		return "SubCapsule"
	case events.EventSubPinEvent:
		return "SubPinEvent"
	case events.EventSubscriptionNotify:
		return "SubscriptionNotify"
	case events.EventToast:
		return "Toast"
	case events.EventUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// --- replay engine ---

func replay(t *testing.T, frames [][]byte) replayResult {
	t.Helper()
	r := replayResult{
		frameCount:   uint64(len(frames)),
		payloadTypes: make(map[string]uint64),
		messageTypes: make(map[string]uint64),
		eventTypes:   make(map[string]uint64),
		unknownTypes: make(map[string]uint64),
		giftGroups:   make(map[string][]giftGroupResult),
	}

	likeAcc := helpers.NewLikeAccumulator()
	giftTracker := helpers.NewGiftStreakTracker()

	for _, raw := range frames {
		frame := &pb.WebcastPushFrame{}
		if err := proto.Unmarshal(raw, frame); err != nil {
			r.decodeFailures++
			continue
		}

		r.payloadTypes[frame.PayloadType]++

		if frame.PayloadType != "msg" {
			continue
		}

		decompressed, err := decompressIfGzipped(frame.Payload)
		if err != nil {
			r.decompressFailures++
			continue
		}

		response := &pb.WebcastResponse{}
		if err := proto.Unmarshal(decompressed, response); err != nil {
			r.decodeFailures++
			continue
		}

		for _, msg := range response.Messages {
			r.messageCount++
			r.messageTypes[msg.Method]++

			evts := events.Decode(msg.Method, msg.Payload)
			for _, evt := range evts {
				r.eventCount++
				etype := eventTypeName(evt)
				r.eventTypes[etype]++

				switch evt.Type {
				case events.EventFollow:
					r.followCount++
				case events.EventShare:
					r.shareCount++
				case events.EventJoin:
					r.joinCount++
				case events.EventLiveEnded:
					r.liveEndedCount++
				case events.EventUnknown:
					if unk, ok := evt.Data.(*events.UnknownEvent); ok {
						r.unknownTypes[unk.Method]++
					}
				}
			}

			if msg.Method == "WebcastLikeMessage" {
				likeMsg := &pb.WebcastLikeMessage{}
				if err := proto.Unmarshal(msg.Payload, likeMsg); err == nil {
					stats := likeAcc.Process(likeMsg)
					r.likeEvents = append(r.likeEvents, likeEventResult{
						wireCount:     likeMsg.Count,
						wireTotal:     likeMsg.Total,
						accTotal:      stats.TotalLikeCount,
						accumulated:   stats.AccumulatedCount,
						wentBackwards: stats.WentBackwards,
					})
				}
			}

			if msg.Method == "WebcastGiftMessage" {
				giftMsg := &pb.WebcastGiftMessage{}
				if err := proto.Unmarshal(msg.Payload, giftMsg); err == nil {
					isCombo := giftMsg.Gift != nil && giftMsg.Gift.Type == 1
					if isCombo {
						r.comboCount++
					} else {
						r.nonComboCount++
					}

					streak := giftTracker.Process(giftMsg)
					if streak.IsFinal {
						r.streakFinals++
					}
					if streak.EventGiftCount < 0 {
						r.negativeDeltas++
					}

					key := strconv.FormatInt(giftMsg.GroupId, 10)
					r.giftGroups[key] = append(r.giftGroups[key], giftGroupResult{
						giftID:       int32(giftMsg.GiftId),
						repeatCount:  giftMsg.RepeatCount,
						delta:        streak.EventGiftCount,
						isFinal:      streak.IsFinal,
						diamondTotal: streak.TotalDiamondCount,
					})
				}
			}
		}
	}

	return r
}

// --- assertions ---

func assertReplay(t *testing.T, name string, r replayResult, m manifest) {
	t.Helper()

	assertEqual(t, name, "frame_count", r.frameCount, m.FrameCount)
	assertEqual(t, name, "message_count", r.messageCount, m.MessageCount)
	assertEqual(t, name, "event_count", r.eventCount, m.EventCount)
	assertEqual(t, name, "decode_failures", r.decodeFailures, m.DecodeFailures)
	assertEqual(t, name, "decompress_failures", r.decompressFailures, m.DecompressFailures)

	assertMapEqual(t, name, "payload_types", r.payloadTypes, m.PayloadTypes)
	assertMapEqual(t, name, "message_types", r.messageTypes, m.MessageTypes)
	assertMapEqual(t, name, "event_types", r.eventTypes, m.EventTypes)

	assertEqual(t, name, "sub_routed.follow", r.followCount, m.SubRouted.Follow)
	assertEqual(t, name, "sub_routed.share", r.shareCount, m.SubRouted.Share)
	assertEqual(t, name, "sub_routed.join", r.joinCount, m.SubRouted.Join)
	assertEqual(t, name, "sub_routed.live_ended", r.liveEndedCount, m.SubRouted.LiveEnded)

	assertMapEqual(t, name, "unknown_types", r.unknownTypes, m.UnknownTypes)

	assertLikeAccumulator(t, name, r, m.LikeAccumulator)
	assertGiftStreaks(t, name, r, m.GiftStreaks)
}

func assertLikeAccumulator(t *testing.T, name string, r replayResult, ml likeManifest) {
	t.Helper()

	assertEqual(t, name, "like.event_count",
		uint64(len(r.likeEvents)), ml.EventCount)

	var backwards uint64
	for _, e := range r.likeEvents {
		if e.wentBackwards {
			backwards++
		}
	}
	assertEqual(t, name, "like.backwards_jumps", backwards, ml.BackwardsJumps)

	if len(r.likeEvents) > 0 {
		last := r.likeEvents[len(r.likeEvents)-1]
		assertEq(t, name, "like.final_max_total", last.accTotal, ml.FinalMaxTotal)
		assertEq(t, name, "like.final_accumulated", last.accumulated, ml.FinalAccumulated)
	}

	accMono := true
	accumMono := true
	for i := 1; i < len(r.likeEvents); i++ {
		if r.likeEvents[i].accTotal < r.likeEvents[i-1].accTotal {
			accMono = false
		}
		if r.likeEvents[i].accumulated < r.likeEvents[i-1].accumulated {
			accumMono = false
		}
	}
	assertEq(t, name, "like.acc_total_monotonic", accMono, ml.AccTotalMonotonic)
	assertEq(t, name, "like.accumulated_monotonic", accumMono, ml.AccumulatedMono)

	if len(r.likeEvents) != len(ml.Events) {
		t.Fatalf("%s: like events length: got %d, want %d",
			name, len(r.likeEvents), len(ml.Events))
	}

	for i, got := range r.likeEvents {
		exp := ml.Events[i]
		assertEq(t, name, fmt.Sprintf("like[%d].wire_count", i),
			got.wireCount, exp.WireCount)
		assertEq(t, name, fmt.Sprintf("like[%d].wire_total", i),
			got.wireTotal, exp.WireTotal)
		assertEq(t, name, fmt.Sprintf("like[%d].acc_total", i),
			got.accTotal, exp.AccTotal)
		assertEq(t, name, fmt.Sprintf("like[%d].accumulated", i),
			got.accumulated, exp.Accumulated)
		assertEq(t, name, fmt.Sprintf("like[%d].went_backwards", i),
			got.wentBackwards, exp.WentBackwards)
	}
}

func assertGiftStreaks(t *testing.T, name string, r replayResult, mg giftManifest) {
	t.Helper()

	assertEqual(t, name, "gift.event_count",
		r.comboCount+r.nonComboCount, mg.EventCount)
	assertEqual(t, name, "gift.combo_count", r.comboCount, mg.ComboCount)
	assertEqual(t, name, "gift.non_combo_count", r.nonComboCount, mg.NonComboCount)
	assertEqual(t, name, "gift.streak_finals", r.streakFinals, mg.StreakFinals)
	assertEqual(t, name, "gift.negative_deltas", r.negativeDeltas, mg.NegativeDeltas)

	assertEq(t, name, "gift.groups.count", len(r.giftGroups), len(mg.Groups))

	for gid, gotEvts := range r.giftGroups {
		expEvts, ok := mg.Groups[gid]
		if !ok {
			t.Fatalf("%s: unexpected gift group %s", name, gid)
		}
		if len(gotEvts) != len(expEvts) {
			t.Fatalf("%s: gift group %s length: got %d, want %d",
				name, gid, len(gotEvts), len(expEvts))
		}
		for i, got := range gotEvts {
			exp := expEvts[i]
			assertEq(t, name, fmt.Sprintf("gift[%s][%d].gift_id", gid, i),
				got.giftID, exp.GiftID)
			assertEq(t, name, fmt.Sprintf("gift[%s][%d].repeat_count", gid, i),
				got.repeatCount, exp.RepeatCount)
			assertEq(t, name, fmt.Sprintf("gift[%s][%d].delta", gid, i),
				got.delta, exp.Delta)
			assertEq(t, name, fmt.Sprintf("gift[%s][%d].is_final", gid, i),
				got.isFinal, exp.IsFinal)
			assertEq(t, name, fmt.Sprintf("gift[%s][%d].diamond_total", gid, i),
				got.diamondTotal, exp.DiamondTotal)
		}
	}
}

// --- generic assertion helpers ---

func assertEqual(t *testing.T, name, field string, got, want uint64) {
	t.Helper()
	if got != want {
		t.Errorf("%s: %s: got %d, want %d", name, field, got, want)
	}
}

func assertEq[T comparable](t *testing.T, name, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: %s: got %v, want %v", name, field, got, want)
	}
}

func assertMapEqual(t *testing.T, name, field string, got, want map[string]uint64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %s: map length got %d, want %d\n  got:  %v\n  want: %v",
			name, field, len(got), len(want), got, want)
		return
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Errorf("%s: %s: missing key %q (want %d)", name, field, k, wv)
			continue
		}
		if gv != wv {
			t.Errorf("%s: %s[%q]: got %d, want %d", name, field, k, gv, wv)
		}
	}
	for k, gv := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: %s: unexpected key %q (got %d)", name, field, k, gv)
		}
	}
}

// --- test runner ---

func runCaptureTest(t *testing.T, name string) {
	t.Helper()

	capPath, manPath, found := findCapturePath(name)
	if !found {
		t.Skipf("testdata not found for %s (set PIRATETOK_TESTDATA or clone live-testdata)", name)
		return
	}

	manData, err := os.ReadFile(manPath)
	if err != nil {
		t.Fatalf("cannot read manifest %s: %v", manPath, err)
	}
	var m manifest
	if err := json.Unmarshal(manData, &m); err != nil {
		t.Fatalf("cannot parse manifest %s: %v", manPath, err)
	}

	frames := readCapture(t, capPath)
	result := replay(t, frames)
	assertReplay(t, name, result, m)
}

// --- test functions ---

func TestReplayCalvinterest6(t *testing.T) {
	runCaptureTest(t, "calvinterest6")
}

func TestReplayHappyhappygaltv(t *testing.T) {
	runCaptureTest(t, "happyhappygaltv")
}

func TestReplayFox4newsdallasfortworth(t *testing.T) {
	runCaptureTest(t, "fox4newsdallasfortworth")
}
