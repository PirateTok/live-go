package events

import (
	pb "github.com/PirateTok/live-go/proto"
	"google.golang.org/protobuf/proto"
)

// Decode decodes a protobuf message by type string and returns one or more events.
// Sub-routed messages (Social, Member, Control) return 2 events: raw + convenience.
func Decode(msgType string, payload []byte) []Event {
	switch msgType {
	// core — sub-routed
	case "WebcastSocialMessage":
		return decodeSocial(payload)
	case "WebcastMemberMessage":
		return decodeMember(payload)
	case "WebcastControlMessage":
		return decodeControl(payload)

	// core — direct
	case "WebcastChatMessage":
		return one(EventChat, payload, &pb.WebcastChatMessage{})
	case "WebcastGiftMessage":
		return one(EventGift, payload, &pb.WebcastGiftMessage{})
	case "WebcastLikeMessage":
		return one(EventLike, payload, &pb.WebcastLikeMessage{})
	case "WebcastRoomUserSeqMessage":
		return one(EventRoomUserSeq, payload, &pb.WebcastRoomUserSeqMessage{})

	// useful
	case "WebcastLiveIntroMessage":
		return one(EventLiveIntro, payload, &pb.WebcastLiveIntroMessage{})
	case "WebcastRoomMessage":
		return one(EventRoomMessage, payload, &pb.WebcastRoomMessage{})
	case "WebcastCaptionMessage":
		return one(EventCaption, payload, &pb.WebcastCaptionMessage{})
	case "WebcastGoalUpdateMessage":
		return one(EventGoalUpdate, payload, &pb.WebcastGoalUpdateMessage{})
	case "WebcastImDeleteMessage":
		return one(EventImDelete, payload, &pb.WebcastImDeleteMessage{})

	// niche
	case "WebcastRankUpdateMessage":
		return one(EventRankUpdate, payload, &pb.WebcastRankUpdateMessage{})
	case "WebcastPollMessage":
		return one(EventPoll, payload, &pb.WebcastPollMessage{})
	case "WebcastEnvelopeMessage":
		return one(EventEnvelope, payload, &pb.WebcastEnvelopeMessage{})
	case "WebcastRoomPinMessage":
		return one(EventRoomPin, payload, &pb.WebcastRoomPinMessage{})
	case "WebcastUnauthorizedMemberMessage":
		return one(EventUnauthorizedMember, payload, &pb.WebcastUnauthorizedMemberMessage{})
	case "WebcastLinkMicMethod":
		return one(EventLinkMicMethod, payload, &pb.WebcastLinkMicMethod{})
	case "WebcastLinkMicBattle":
		return one(EventLinkMicBattle, payload, &pb.WebcastLinkMicBattle{})
	case "WebcastLinkMicArmies":
		return one(EventLinkMicArmies, payload, &pb.WebcastLinkMicArmies{})
	case "WebcastLinkMessage":
		return one(EventLinkMessage, payload, &pb.WebcastLinkMessage{})
	case "WebcastLinkLayerMessage":
		return one(EventLinkLayer, payload, &pb.WebcastLinkLayerMessage{})
	case "WebcastLinkMicLayoutStateMessage":
		return one(EventLinkMicLayoutState, payload, &pb.WebcastLinkMicLayoutStateMessage{})
	case "WebcastGiftPanelUpdateMessage":
		return one(EventGiftPanelUpdate, payload, &pb.WebcastGiftPanelUpdateMessage{})
	case "WebcastInRoomBannerMessage":
		return one(EventInRoomBanner, payload, &pb.WebcastInRoomBannerMessage{})
	case "WebcastGuideMessage":
		return one(EventGuide, payload, &pb.WebcastGuideMessage{})

	// extended
	case "WebcastEmoteChatMessage":
		return one(EventEmoteChat, payload, &pb.WebcastEmoteChatMessage{})
	case "WebcastQuestionNewMessage":
		return one(EventQuestionNew, payload, &pb.WebcastQuestionNewMessage{})
	case "WebcastSubNotifyMessage":
		return one(EventSubNotify, payload, &pb.WebcastSubNotifyMessage{})
	case "WebcastBarrageMessage":
		return one(EventBarrage, payload, &pb.WebcastBarrageMessage{})
	case "WebcastHourlyRankMessage":
		return one(EventHourlyRank, payload, &pb.WebcastHourlyRankMessage{})
	case "WebcastMsgDetectMessage":
		return one(EventMsgDetect, payload, &pb.WebcastMsgDetectMessage{})
	case "WebcastLinkMicFanTicketMethod":
		return one(EventLinkMicFanTicket, payload, &pb.WebcastLinkMicFanTicketMethod{})
	case "WebcastRoomVerifyMessage", "RoomVerifyMessage":
		return one(EventRoomVerify, payload, &pb.RoomVerifyMessage{})
	case "WebcastOecLiveShoppingMessage":
		return one(EventOecLiveShopping, payload, &pb.WebcastOecLiveShoppingMessage{})
	case "WebcastGiftBroadcastMessage":
		return one(EventGiftBroadcast, payload, &pb.WebcastGiftBroadcastMessage{})
	case "WebcastRankTextMessage":
		return one(EventRankText, payload, &pb.WebcastRankTextMessage{})
	case "WebcastGiftDynamicRestrictionMessage":
		return one(EventGiftDynamicRestriction, payload, &pb.WebcastGiftDynamicRestrictionMessage{})
	case "WebcastViewerPicksUpdateMessage":
		return one(EventViewerPicksUpdate, payload, &pb.WebcastViewerPicksUpdateMessage{})

	// secondary
	case "WebcastSystemMessage":
		return one(EventSystemMessage, payload, &pb.WebcastSystemMessage{})
	case "WebcastLiveGameIntroMessage":
		return one(EventLiveGameIntro, payload, &pb.WebcastLiveGameIntroMessage{})
	case "WebcastAccessControlMessage":
		return one(EventAccessControl, payload, &pb.WebcastAccessControlMessage{})
	case "WebcastAccessRecallMessage":
		return one(EventAccessRecall, payload, &pb.WebcastAccessRecallMessage{})
	case "WebcastAlertBoxAuditResultMessage":
		return one(EventAlertBoxAuditResult, payload, &pb.WebcastAlertBoxAuditResultMessage{})
	case "WebcastBindingGiftMessage":
		return one(EventBindingGift, payload, &pb.WebcastBindingGiftMessage{})
	case "WebcastBoostCardMessage":
		return one(EventBoostCard, payload, &pb.WebcastBoostCardMessage{})
	case "WebcastBottomMessage":
		return one(EventBottomMessage, payload, &pb.WebcastBottomMessage{})
	case "WebcastGameRankNotifyMessage":
		return one(EventGameRankNotify, payload, &pb.WebcastGameRankNotifyMessage{})
	case "WebcastGiftPromptMessage":
		return one(EventGiftPrompt, payload, &pb.WebcastGiftPromptMessage{})
	case "WebcastLinkStateMessage":
		return one(EventLinkState, payload, &pb.WebcastLinkStateMessage{})
	case "WebcastLinkMicBattlePunishFinish":
		return one(EventLinkMicBattlePunishFinish, payload, &pb.WebcastLinkMicBattlePunishFinish{})
	case "WebcastLinkmicBattleTaskMessage":
		return one(EventLinkmicBattleTask, payload, &pb.WebcastLinkmicBattleTaskMessage{})
	case "WebcastMarqueeAnnouncementMessage":
		return one(EventMarqueeAnnouncement, payload, &pb.WebcastMarqueeAnnouncementMessage{})
	case "WebcastNoticeMessage":
		return one(EventNotice, payload, &pb.WebcastNoticeMessage{})
	case "WebcastNotifyMessage":
		return one(EventNotify, payload, &pb.WebcastNotifyMessage{})
	case "WebcastPartnershipDropsUpdateMessage":
		return one(EventPartnershipDropsUpdate, payload, &pb.WebcastPartnershipDropsUpdateMessage{})
	case "WebcastPartnershipGameOfflineMessage":
		return one(EventPartnershipGameOffline, payload, &pb.WebcastPartnershipGameOfflineMessage{})
	case "WebcastPartnershipPunishMessage":
		return one(EventPartnershipPunish, payload, &pb.WebcastPartnershipPunishMessage{})
	case "WebcastPerceptionMessage":
		return one(EventPerception, payload, &pb.WebcastPerceptionMessage{})
	case "WebcastSpeakerMessage":
		return one(EventSpeaker, payload, &pb.WebcastSpeakerMessage{})
	case "WebcastSubCapsuleMessage":
		return one(EventSubCapsule, payload, &pb.WebcastSubCapsuleMessage{})
	case "WebcastSubPinEventMessage":
		return one(EventSubPinEvent, payload, &pb.WebcastSubPinEventMessage{})
	case "WebcastSubscriptionNotifyMessage":
		return one(EventSubscriptionNotify, payload, &pb.WebcastSubscriptionNotifyMessage{})
	case "WebcastToastMessage":
		return one(EventToast, payload, &pb.WebcastToastMessage{})

	default:
		return []Event{{Type: EventUnknown, Data: &UnknownEvent{Method: msgType, Payload: payload}}}
	}
}

func decodeSocial(payload []byte) []Event {
	msg := &pb.WebcastSocialMessage{}
	if err := proto.Unmarshal(payload, msg); err != nil {
		return []Event{{Type: EventUnknown, Data: &UnknownEvent{Method: "WebcastSocialMessage", Payload: payload}}}
	}
	evts := []Event{{Type: EventSocial, Data: msg}}
	switch msg.Action {
	case 1:
		evts = append(evts, Event{Type: EventFollow, Data: msg})
	case 2, 3, 4, 5:
		evts = append(evts, Event{Type: EventShare, Data: msg})
	}
	return evts
}

func decodeMember(payload []byte) []Event {
	msg := &pb.WebcastMemberMessage{}
	if err := proto.Unmarshal(payload, msg); err != nil {
		return []Event{{Type: EventUnknown, Data: &UnknownEvent{Method: "WebcastMemberMessage", Payload: payload}}}
	}
	evts := []Event{{Type: EventMember, Data: msg}}
	if msg.Action == 1 {
		evts = append(evts, Event{Type: EventJoin, Data: msg})
	}
	return evts
}

func decodeControl(payload []byte) []Event {
	msg := &pb.WebcastControlMessage{}
	if err := proto.Unmarshal(payload, msg); err != nil {
		return []Event{{Type: EventUnknown, Data: &UnknownEvent{Method: "WebcastControlMessage", Payload: payload}}}
	}
	evts := []Event{{Type: EventControl, Data: msg}}
	if msg.Action == 3 {
		evts = append(evts, Event{Type: EventLiveEnded, Data: msg})
	}
	return evts
}

func one(typ EventType, payload []byte, msg proto.Message) []Event {
	if err := proto.Unmarshal(payload, msg); err != nil {
		return []Event{{Type: EventUnknown, Data: &UnknownEvent{Method: string(msg.ProtoReflect().Descriptor().FullName()), Payload: payload}}}
	}
	return []Event{{Type: typ, Data: msg}}
}
