package meshcom

import "strings"

// Store-and-forward custody notices (":sto") are control frames sent by a
// mailbox node to the original sender of a message it holds. They follow the
// wire format "%-9.9s:sto%03u %s" — the addressee padded to nine bytes, the
// ":sto" tag anchored at byte 9, a three-digit transport sequence, then the
// held destination as readable text (see docs/chat.md).
const (
	storeNoticeTag          = ":sto"
	storeNoticeTagOffset    = 9
	storeNoticeSequenceLen  = 3
	storeNoticeMinLength    = storeNoticeTagOffset + len(storeNoticeTag) + storeNoticeSequenceLen
	storeNoticeSequenceFrom = storeNoticeTagOffset + len(storeNoticeTag)
)

// IsStoreNotice reports whether msg is a store-and-forward custody notice.
// The check mirrors the firmware parser: the ":sto" tag must sit exactly at
// byte 9 and be followed by three decimal digits, and the payload must not
// carry a transport sequence or an ACK/reject tag.
func IsStoreNotice(msg string) bool {
	if len(msg) < storeNoticeMinLength {
		return false
	}
	if msg[storeNoticeTagOffset:storeNoticeSequenceFrom] != storeNoticeTag {
		return false
	}
	for _, digit := range msg[storeNoticeSequenceFrom:storeNoticeMinLength] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	if strings.Contains(msg, "{") {
		return false
	}
	lower := strings.ToLower(msg)
	return !strings.Contains(lower, ":ack") && !strings.Contains(lower, ":rej")
}
