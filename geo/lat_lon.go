package geo

import (
	"fmt"
	"math"
	"strconv"
)

type Latitude float64
type Longitude float64

func ParseLatitude(s string) (lat Latitude, err error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Latitude(math.NaN()), err
	}
	return LatitudeFromFloat64(v)
}
func LatitudeFromFloat64(v float64) (Latitude, error) {
	if v < -90 || v > 90 {
		ne := fmt.Errorf("latitude value out of range: %v", v)
		return Latitude(math.NaN()), ne
	}
	return Latitude(v), nil
}
func (l Latitude) IsValid() bool {
	return -90 <= l && l <= 90
}
func (l Latitude) Clamp() Latitude {
	if l < -90 {
		return -90
	}
	if l > 90 {
		return 90
	}
	return l
}
func (v Latitude) FormatDMS() string {
	return formatDMS(float64(v), "S", "N")
}
func (l Latitude) ToRadians() float64 {
	return float64(l) * math.Pi / 180
}
func (l Latitude) Sin() float64 {
	return math.Sin(l.ToRadians())
}
// Returns the (approximate) radius of the earth at this latitude.
func (l Latitude) EarthRadius() Meters {
	// 6378137 is the earth's equatorial radius (meters).  The radius at the
	// poles is about 21384.7 meters smaller than the equatorial radius.
	return Meters(6378137 - 21384.7*math.Abs(math.Sin(l.ToRadians())))
}
// Returns the length of the arc (constant longitude) between two latitudes,
// never negative.
func (lat1 Latitude) ArcLength(lat2 Latitude) Meters {
	if lat1 > lat2 {
		lat1, lat2 = lat2, lat1
	}
	fn := func(l1, l2 Latitude) Meters {
		mid := (l1 + l2) / 2
		r := mid.EarthRadius()
		arc := l2 - l1
		arcFrac := float64(arc / 360)
		circum := float64(r) * math.Pi * 2
		return Meters(circum * arcFrac)
	}
	d := lat2 - lat1
	if d < 0.5 {
		return fn(lat1, lat2)
	}
	var total Meters = 0
	const step = 0.25
	s := lat1
	for {
		t := s + step
		if t >= lat2 {
			return total + fn(s, lat2)
		}
		total += fn(s, t)
		s = t
	}
}
// Area of earth north of this line of latitude.
func (l Latitude) AreaOfCap() MetersSq {
	r := float64(l.EarthRadius())
	return MetersSq(2 * math.Pi * r * r * (1 - l.Sin()))
}

////////////////////////////////////////////////////////////////////////////////

func ParseLongitude(s string) (lon Longitude, err error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Longitude(math.NaN()), err
	}
	return LongitudeFromFloat64(v)
}
func LongitudeFromFloat64(v float64) (Longitude, error) {
	if v < -180 || v > 180 {
		ne := fmt.Errorf("longitude value out of range: %v", v)
		return Longitude(math.NaN()), ne
	}
	return Longitude(v), nil
}
func (l Longitude) IsValid() bool {
	return -180 <= l && l <= 180
}
func (l Longitude) Clamp() Longitude {
	if l < -180 {
		return -180
	}
	if l > 180 {
		return 180
	}
	return l
}
func (v Longitude) FormatDMS() string {
	return formatDMS(float64(v), "W", "E")
}
func (l Longitude) ToRadians() float64 {
	return float64(l) * math.Pi / 180
}
