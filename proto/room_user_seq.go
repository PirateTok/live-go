package proto

import "sort"

// TopViewers returns the top-viewers box next to the viewer counter (usually
// the top 3 ranked by contribution score). Entries without a decoded user are
// skipped; the rest come back sorted by rank. No cookies needed — it rides on
// every RoomUserSeq event.
func (x *WebcastRoomUserSeqMessage) TopViewers() []*WebcastRoomUserSeqMessage_Contributor {
	top := make([]*WebcastRoomUserSeqMessage_Contributor, 0, len(x.GetRanksList()))
	for _, c := range x.GetRanksList() {
		if c.GetUser() != nil {
			top = append(top, c)
		}
	}
	sort.SliceStable(top, func(i, j int) bool { return top[i].GetRank() < top[j].GetRank() })
	return top
}
