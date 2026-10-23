package routing

// polylinePrecision is the encoded-polyline scale used by Google and OSRM (5 decimals).
const polylinePrecision = 1e5

// DecodePolyline turns an encoded polyline (precision 5) into points; a
// malformed string yields the points decoded so far.
func DecodePolyline(s string) []Point {
	var out []Point
	lat, lng := 0, 0
	for i := 0; i < len(s); {
		dLat, n := decodeValue(s, i)
		if n == 0 {
			return out
		}
		i += n
		dLng, m := decodeValue(s, i)
		if m == 0 {
			return out
		}
		i += m
		lat, lng = lat+dLat, lng+dLng
		out = append(out, Point{Lat: float64(lat) / polylinePrecision, Lng: float64(lng) / polylinePrecision})
	}
	return out
}

// decodeValue reads one zig-zag varint from s at i; it returns the value and the bytes used (0 = malformed).
func decodeValue(s string, i int) (int, int) {
	const chunkBits, more, offset = 5, 0x20, 63
	result, shift, used := 0, 0, 0
	for i+used < len(s) {
		b := int(s[i+used]) - offset
		used++
		result |= (b & (more - 1)) << shift
		shift += chunkBits
		if b < more {
			if result&1 != 0 {
				return ^(result >> 1), used
			}
			return result >> 1, used
		}
	}
	return 0, 0
}
