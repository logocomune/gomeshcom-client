package stats

import (
	"fmt"
	"math"
)

const (
	distanceBinSizeKm = 5.0
	distanceMaxKm     = 100.0
)

// HaversineKm returns the great-circle distance in kilometres between two
// geographic coordinates. Exported for use in tests and the collector.
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// DistanceBucketLabel returns the histogram bucket label for a given distance
// in kilometres, using distanceBinSizeKm-wide bins up to distanceMaxKm.
// Example: 7.3 → "5-10", 102 → "100+".
func DistanceBucketLabel(km float64) string {
	if km >= distanceMaxKm {
		return fmt.Sprintf("%d+", int(distanceMaxKm))
	}
	bin := int(km / distanceBinSizeKm)
	lo := bin * int(distanceBinSizeKm)
	hi := lo + int(distanceBinSizeKm)
	return fmt.Sprintf("%d-%d", lo, hi)
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180.0
}
