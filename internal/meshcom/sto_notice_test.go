package meshcom

import (
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestIsStoreNotice(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "canonical notice",
			msg:  "QQ1OWN-1 :sto156 QQ1PEER-2",
			want: true,
		},
		{
			name: "short callsign padded to nine bytes",
			msg:  "QQ1AB    :sto001 QQ1PEER-2",
			want: true,
		},
		{
			name: "notice without destination call",
			msg:  "QQ1OWN-1 :sto017",
			want: true,
		},
		{
			name: "tag not anchored at byte nine",
			msg:  "look at this :sto156 QQ1PEER-2",
			want: false,
		},
		{
			name: "tag too early",
			msg:  "QQ1OWN:sto156 QQ1PEER-2",
			want: false,
		},
		{
			name: "missing digits",
			msg:  "QQ1OWN-1 :sto QQ1PEER-2",
			want: false,
		},
		{
			name: "non numeric sequence",
			msg:  "QQ1OWN-1 :stoabc QQ1PEER-2",
			want: false,
		},
		{
			name: "truncated sequence",
			msg:  "QQ1OWN-1 :sto15",
			want: false,
		},
		{
			name: "payload too short",
			msg:  ":sto156 QQ1PEER-2",
			want: false,
		},
		{
			name: "carries transport sequence tag",
			msg:  "QQ1OWN-1 :sto156 QQ1PEER-2{156",
			want: false,
		},
		{
			name: "carries ack tag",
			msg:  "QQ1OWN-1 :sto156 QQ1PEER-2 :ack571",
			want: false,
		},
		{
			name: "carries reject tag",
			msg:  "QQ1OWN-1 :sto156 QQ1PEER-2 :REJ571",
			want: false,
		},
		{
			name: "uppercase tag is not a notice",
			msg:  "QQ1OWN-1 :STO156 QQ1PEER-2",
			want: false,
		},
		{
			name: "empty message",
			msg:  "",
			want: false,
		},
		{
			name: "ordinary chat message",
			msg:  "hello there",
			want: false,
		},
		{name: "Unicode at UTF-16 offset nine", msg: "é12345678:sto156 QQ1PEER-2", want: false},
		{name: "Unicode at UTF-8 offset nine", msg: "é1234567:sto156 QQ1PEER-2", want: true},
		{name: "astral Unicode at UTF-16 offset nine", msg: "😀1234567:sto156 QQ1PEER-2", want: false},
		{name: "astral Unicode at UTF-8 offset nine", msg: "😀12345:sto156 QQ1PEER-2", want: true},
		{name: "Unicode folded ack tag", msg: "QQ1OWN-1 :sto156 :acK571", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStoreNotice(tt.msg); got != tt.want {
				t.Fatalf("IsStoreNotice(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

// TestIsStoreNoticeAcceptsAnySequence pins the parser to the wire format
// "%-9.9s:sto%03u %s": the addressee occupies nine bytes and the sequence is
// always three decimal digits.
func TestIsStoreNoticeAcceptsAnySequence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		addressee := rapid.StringMatching(`[A-Z0-9\-]{1,9}`).Draw(t, "addressee")
		sequence := rapid.IntRange(0, 999).Draw(t, "sequence")
		destination := rapid.StringMatching(`[A-Z0-9\-]{1,9}`).Draw(t, "destination")

		payload := padToNineBytes(addressee) + ":sto" + padSequence(sequence)
		if !IsStoreNotice(payload) {
			t.Fatalf("IsStoreNotice(%q) = false, want true", payload)
		}
		if !IsStoreNotice(payload + " " + destination) {
			t.Fatalf("IsStoreNotice(%q) = false, want true", payload+" "+destination)
		}
	})
}

func padToNineBytes(addressee string) string {
	return addressee + strings.Repeat(" ", 9-len(addressee))
}

func padSequence(sequence int) string {
	digits := strconv.Itoa(sequence)
	return strings.Repeat("0", 3-len(digits)) + digits
}

func FuzzIsStoreNotice(f *testing.F) {
	seeds := []string{
		"QQ1OWN-1 :sto156 QQ1PEER-2",
		"QQ1OWN-1 :sto000",
		"QQ1AB    :sto999 QQ1PEER-2",
		"IK5FTL-11:sto156 IK5FCK-10",
		"DK5EN-93 :sto017 DK5EN-14",
		"look at this :sto156 QQ1PEER-2",
		"QQ1OWN-1 :sto156 QQ1PEER-2{156",
		"é12345678:sto156 QQ1PEER-2",
		"é1234567:sto156 QQ1PEER-2",
		"😀1234567:sto156 QQ1PEER-2",
		"😀12345:sto156 QQ1PEER-2",
		"QQ1OWN-1 :sto156 :acK571",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, msg string) {
		if IsStoreNotice(msg) && (strings.Contains(msg, "{") || len(msg) < 16) {
			t.Fatalf("IsStoreNotice(%q) = true for a payload outside the wire contract", msg)
		}
	})
}
