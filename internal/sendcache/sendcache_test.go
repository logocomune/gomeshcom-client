package sendcache

import (
	"testing"
	"time"
)

func TestReserveNewKey(t *testing.T) {
	c := New(100 * time.Millisecond)
	if !c.Reserve("QQ1ABC-1", "hello") {
		t.Fatal("first Reserve should return true")
	}
}

func TestReserveDuplicateWithinTTL(t *testing.T) {
	c := New(100 * time.Millisecond)
	c.Reserve("QQ1ABC-1", "hello")
	if c.Reserve("QQ1ABC-1", "hello") {
		t.Fatal("duplicate within TTL should return false")
	}
}

func TestReserveAfterTTLExpiry(t *testing.T) {
	ttl := 50 * time.Millisecond
	c := New(ttl)
	c.Reserve("QQ1ABC-1", "hello")
	time.Sleep(ttl + 20*time.Millisecond)
	if !c.Reserve("QQ1ABC-1", "hello") {
		t.Fatal("Reserve after TTL expiry should return true")
	}
}

func TestReserveIndependentKeys(t *testing.T) {
	cases := []struct {
		dst string
		msg string
	}{
		{"QQ1ABC-1", "hello"},
		{"QQ1ABC-2", "hello"},
		{"QQ1ABC-1", "world"},
		{"*", "broadcast"},
	}
	c := New(100 * time.Millisecond)
	for _, tc := range cases {
		if !c.Reserve(tc.dst, tc.msg) {
			t.Errorf("Reserve(%q, %q) should be new key, got false", tc.dst, tc.msg)
		}
	}
}

func TestReserveZeroTTLAlwaysTrue(t *testing.T) {
	c := New(0)
	c.Reserve("dst", "msg")
	if !c.Reserve("dst", "msg") {
		t.Fatal("TTL=0 disables dedup; Reserve should always return true")
	}
}
