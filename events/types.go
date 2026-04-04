package events

// EventType identifies the kind of event.
type EventType int

const (
	// lifecycle
	EventConnected    EventType = iota
	EventReconnecting
	EventDisconnected

	// core
	EventChat
	EventGift
	EventLike
	EventMember
	EventSocial
	EventRoomUserSeq
	EventControl

	// convenience sub-routed
	EventFollow
	EventShare
	EventJoin
	EventLiveEnded

	// useful
	EventLiveIntro
	EventRoomMessage
	EventCaption
	EventGoalUpdate
	EventImDelete

	// niche
	EventRankUpdate
	EventPoll
	EventEnvelope
	EventRoomPin
	EventUnauthorizedMember
	EventLinkMicMethod
	EventLinkMicBattle
	EventLinkMicArmies
	EventLinkMessage
	EventLinkLayer
	EventLinkMicLayoutState
	EventGiftPanelUpdate
	EventInRoomBanner
	EventGuide

	// extended
	EventEmoteChat
	EventQuestionNew
	EventSubNotify
	EventBarrage
	EventHourlyRank
	EventMsgDetect
	EventLinkMicFanTicket
	EventRoomVerify
	EventOecLiveShopping
	EventGiftBroadcast
	EventRankText
	EventGiftDynamicRestriction
	EventViewerPicksUpdate

	// secondary
	EventSystemMessage
	EventLiveGameIntro
	EventAccessControl
	EventAccessRecall
	EventAlertBoxAuditResult
	EventBindingGift
	EventBoostCard
	EventBottomMessage
	EventGameRankNotify
	EventGiftPrompt
	EventLinkState
	EventLinkMicBattlePunishFinish
	EventLinkmicBattleTask
	EventMarqueeAnnouncement
	EventNotice
	EventNotify
	EventPartnershipDropsUpdate
	EventPartnershipGameOffline
	EventPartnershipPunish
	EventPerception
	EventSpeaker
	EventSubCapsule
	EventSubPinEvent
	EventSubscriptionNotify
	EventToast

	// catch-all
	EventUnknown
)

// Event represents a decoded TikTok Live event.
type Event struct {
	Type    EventType
	Data    interface{}
	RoomID  string // only for EventConnected
}

// UnknownEvent wraps an unrecognized message type with raw payload.
type UnknownEvent struct {
	Method  string
	Payload []byte
}
