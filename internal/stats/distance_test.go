package stats

import (
	"math"
	"math/rand"
	"testing"
	"testing/quick"
)

func TestHaversineKmSamePoint(t *testing.T) {
	coords := []struct{ lat, lng float64 }{
		{0, 0},
		{43.7, 11.2},
		{-33.8, 151.2},
		{90, 0},
	}
	for _, c := range coords {
		got := HaversineKm(c.lat, c.lng, c.lat, c.lng)
		if got != 0.0 {
			t.Errorf("HaversineKm(%v,%v,%v,%v) = %v, want 0", c.lat, c.lng, c.lat, c.lng, got)
		}
	}
}

func TestHaversineKmSymmetry(t *testing.T) {
	f := func(lat1, lng1, lat2, lng2 float64) bool {
		// Clamp to valid coordinate ranges.
		lat1 = clampLat(lat1)
		lng1 = clampLng(lng1)
		lat2 = clampLat(lat2)
		lng2 = clampLng(lng2)
		d1 := HaversineKm(lat1, lng1, lat2, lng2)
		d2 := HaversineKm(lat2, lng2, lat1, lng1)
		return math.Abs(d1-d2) < 1e-9
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(42))}); err != nil {
		t.Fatal(err)
	}
}

func TestHaversineKmNonNegative(t *testing.T) {
	f := func(lat1, lng1, lat2, lng2 float64) bool {
		lat1 = clampLat(lat1)
		lng1 = clampLng(lng1)
		lat2 = clampLat(lat2)
		lng2 = clampLng(lng2)
		return HaversineKm(lat1, lng1, lat2, lng2) >= 0
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(43))}); err != nil {
		t.Fatal(err)
	}
}

// Known distance: Florence (43.7714°N 11.2542°E) to Rome (41.8967°N 12.4822°E) ≈ 232 km.
func TestHaversineKmKnownDistance(t *testing.T) {
	got := HaversineKm(43.7714, 11.2542, 41.8967, 12.4822)
	const want = 232.0
	if math.Abs(got-want) > 5.0 {
		t.Errorf("Florence–Rome = %.1f km, want %.1f ± 5", got, want)
	}
}

var distanceBucketCases = []struct {
	km    float64
	label string
}{
	{0, "0-5"},
	{2.5, "0-5"},
	{5, "5-10"},
	{7.3, "5-10"},
	{49.9, "45-50"},
	{99.9, "95-100"},
	{100, "100+"},
	{150, "100+"},
}

func TestDistanceBucketLabel(t *testing.T) {
	for _, tc := range distanceBucketCases {
		got := DistanceBucketLabel(tc.km)
		if got != tc.label {
			t.Errorf("DistanceBucketLabel(%.1f) = %q, want %q", tc.km, got, tc.label)
		}
	}
}

// ---- helpers ----------------------------------------------------------------

func clampLat(v float64) float64 {
	v = math.Mod(v, 180)
	if v > 90 {
		v -= 180
	} else if v < -90 {
		v += 180
	}
	return v
}

func clampLng(v float64) float64 {
	v = math.Mod(v, 360)
	if v > 180 {
		v -= 360
	} else if v < -180 {
		v += 360
	}
	return v
}
