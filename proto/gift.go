package proto

// IsComboGift reports whether the gift streaks (gift type 1): TikTok sends
// several events with a running RepeatCount until RepeatEnd.
func (x *WebcastGiftMessage) IsComboGift() bool {
	return x.GetGift().GetType() == 1
}

// IsStreakOver reports whether this event closes the gift: always true for
// non-combo gifts, true on RepeatEnd == 1 for combos.
func (x *WebcastGiftMessage) IsStreakOver() bool {
	return !x.IsComboGift() || x.GetRepeatEnd() == 1
}

// DiamondTotal is diamonds per gift × RepeatCount (at least 1).
func (x *WebcastGiftMessage) DiamondTotal() int64 {
	count := int64(x.GetRepeatCount())
	if count < 1 {
		count = 1
	}
	return int64(x.GetGift().GetDiamondCount()) * count
}
