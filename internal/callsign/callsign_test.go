package callsign_test

import (
	"testing"
	"unicode/utf8"

	"github.com/logocomune/gomeshcom-client/internal/callsign"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"iu5pmp", "IU5PMP"},
		{" IU5PMP-1 ", "IU5PMP-1"},
		{"  qQ1ABC-10  ", "QQ1ABC-10"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := callsign.Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	calls := []string{"IU5PMP-1", "QQ0XX-1", "ABC123", "ZZ9ZZZ-9"}
	for _, c := range calls {
		t.Run(c, func(t *testing.T) {
			once := callsign.Normalize(c)
			twice := callsign.Normalize(once)
			if once != twice {
				t.Errorf("Normalize not idempotent: %q → %q → %q", c, once, twice)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		// Valid
		{"IU5PMP", true},
		{"IU5PMP-1", true},
		{"IU5PMP-10", true},
		{"QQ0XX-1", true},
		{"ABC", true},
		{"ABCDEFGHIJ", true}, // 10 chars
		{"A1B2C3D4", true},
		// Lowercase input is normalized first
		{"iu5pmp-1", true},
		// Invalid
		{"", false},
		{"AB", false},          // too short
		{"ABCDEFGHIJK", false}, // 11 chars
		{"IU5PMP-100", false},  // SSID > 99
		{"IU5PMP-", false},     // dangling dash
		{"IU5PMP-A", false},    // non-numeric SSID
		{"IU5PMP!", false},     // special char
		{"  ", false},          // whitespace only
		{"IU5 PMP", false},     // internal space
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := callsign.IsValid(tt.in); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func FuzzIsValid(f *testing.F) {
	seeds := []string{
		"IU5PMP-1",
		"QQ0XX-1",
		"ABC",
		"",
		"-1",
		"A!B",
		"ABCDEFGHIJK",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic.
		got := callsign.IsValid(input)
		norm := callsign.Normalize(input)

		// If IsValid returns true, normalized input must match Pattern.
		if got && !callsign.Pattern.MatchString(norm) {
			t.Errorf("IsValid(%q)=true but Pattern does not match normalized %q", input, norm)
		}
		// If input is valid ASCII within pattern bounds, roundtrip must hold.
		if utf8.ValidString(input) && callsign.IsValid(norm) {
			if !callsign.IsValid(callsign.Normalize(norm)) {
				t.Errorf("Normalize not idempotent for valid callsign %q", norm)
			}
		}
	})
}
