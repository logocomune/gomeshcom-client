package chatlog

import (
	"testing"
)

var isDMCases = []struct {
	dst  string
	want bool
}{
	{"", false},
	{"*", false},
	{"0", false},
	{"9", false},
	{"123", false},
	{"IU5PMP-1", true},
	{"CALL", true},
	{"abc", true},
}

func TestIsDMTable(t *testing.T) {
	for _, tc := range isDMCases {
		got := IsDM(tc.dst)
		if got != tc.want {
			t.Errorf("IsDM(%q) = %v, want %v", tc.dst, got, tc.want)
		}
	}
}

// FuzzIsDM ensures IsDM never panics on arbitrary input.
func FuzzIsDM(f *testing.F) {
	for _, tc := range isDMCases {
		f.Add(tc.dst)
	}
	f.Fuzz(func(t *testing.T, dst string) {
		// Must not panic. Result is either true or false; no oracle, just crash safety.
		_ = IsDM(dst)
	})
}
