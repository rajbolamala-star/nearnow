package geo

import "math"

const earthRadiusMiles = 3958.8

// Distance calculates distance between two coordinates in miles
// using the Haversine formula
func Distance(lat1, lng1, lat2, lng2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMiles * c
}

// BoundingBox returns lat/lng bounds for a given radius in miles
// Used to filter events before precise distance calculation
func BoundingBox(lat, lng, radiusMiles float64) (minLat, maxLat, minLng, maxLng float64) {
	latDelta := radiusMiles / earthRadiusMiles * 180 / math.Pi
	lngDelta := radiusMiles / (earthRadiusMiles * math.Cos(lat*math.Pi/180)) * 180 / math.Pi
	return lat - latDelta, lat + latDelta, lng - lngDelta, lng + lngDelta
}

// IsWithinRadius checks if a point is within radius miles of a center
func IsWithinRadius(centerLat, centerLng, pointLat, pointLng, radiusMiles float64) bool {
	return Distance(centerLat, centerLng, pointLat, pointLng) <= radiusMiles
}
