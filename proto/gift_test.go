package proto

import "testing"

func TestGiftHelpers(t *testing.T) {
	combo := &WebcastGiftMessage{Gift: &GiftStruct{Type: 1, DiamondCount: 5}, RepeatCount: 7}
	if !combo.IsComboGift() || combo.IsStreakOver() || combo.DiamondTotal() != 35 {
		t.Fatalf("mid-streak combo: combo=%v over=%v total=%d", combo.IsComboGift(), combo.IsStreakOver(), combo.DiamondTotal())
	}
	combo.RepeatEnd = 1
	if !combo.IsStreakOver() {
		t.Fatal("combo with RepeatEnd=1 must be over")
	}
	single := &WebcastGiftMessage{Gift: &GiftStruct{Type: 2, DiamondCount: 100}}
	if single.IsComboGift() || !single.IsStreakOver() || single.DiamondTotal() != 100 {
		t.Fatalf("non-combo: combo=%v over=%v total=%d", single.IsComboGift(), single.IsStreakOver(), single.DiamondTotal())
	}
	if (&WebcastGiftMessage{}).DiamondTotal() != 0 {
		t.Fatal("no gift details must total 0")
	}
}
