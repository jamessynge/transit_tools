package geo

import (
	"fmt"
	"math"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/util"
)


type Location struct {
	Lat Latitude
	Lon Longitude
}

func LocationFromFloat64s(lat64, lon64 float64) (loc Location, err error) {
	loc.Lat, err = LatitudeFromFloat64(lat64)
	if err == nil {
		loc.Lon, err = LongitudeFromFloat64(lon64)
	}
	return loc, err
}

func (u Location) String() string {
	return fmt.Sprintf("(%f, %f)", u.Lat, u.Lon)
}

func (u Location) SameLocation(v Location) bool {
	return u.Lat == v.Lat && u.Lon == v.Lon
}

func (a Location) SouthToNorthLess(b Location) bool {
	return a.Lat < b.Lat
}
func SouthToNorthLess(a, b Location) bool {
	return a.SouthToNorthLess(b)
}
func SortSouthToNorth(s []Location) {
	less := func(i, j int) bool {
		return SouthToNorthLess(s[i], s[j])
	}
	swap := func(i, j int) {
		s[i], s[j] = s[j], s[i]
	}
	util.Sort3(len(s), less, swap)
}

func (a Location) WestToEastLess(b Location) bool {
	return a.Lon < b.Lon
}
func WestToEastLess(a, b Location) bool {
	return a.Lon < b.Lon
}
func SortWestToEast(s []Location) {
	less := func(i, j int) bool {
		return WestToEastLess(s[i], s[j])
	}
	swap := func(i, j int) {
		s[i], s[j] = s[j], s[i]
	}
	util.Sort3(len(s), less, swap)
}

// Uses Haversine formula to compute the great circle distance between
// loc1 and loc2, and the initial heading at loc1 of the great circle
// path to loc2. The distance is in meters, and the heading is in degrees,
// with north = 0, east = 90.
// From: "Virtues of the Haversine", R. W. Sinnott,
//       Sky and Telescope, vol 68, no 2, 1984
// Via: http://www.movable-type.co.uk/scripts/latlong.html
//
//       a = sin²(Δφ/2) + cos φ₁ ⋅ cos φ₂ ⋅ sin²(Δλ/2)
//       c = 2 ⋅ atan2( √a, √(1−a) )
//       d = R ⋅ c
//
//       where φ is latitude, λ is longitude, R is earth’s radius
//       (mean radius = 6,371km); note that angles need to be in
//       radians to pass to trig functions!
func (loc1 Location) DistanceAndHeadingTo(loc2 Location) (Meters, HeadingF) {
	deltaLat := (loc2.Lat - loc1.Lat).ToRadians()
	deltaLon := (loc2.Lon - loc1.Lon).ToRadians()
	u := math.Sin(deltaLat / 2)
	v := math.Sin(deltaLon / 2)

	lat1 := toRadians(float64(loc1.Lat))
	lat2 := toRadians(float64(loc2.Lat))
	c1 := math.Cos(lat1)
	c2 := math.Cos(lat2)

	a := u*u + v*v*c1*c2
	greatCircleRadians := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	var radius Meters
	if greatCircleRadians < 0.5 {
		midLat := (loc1.Lat + loc2.Lat) / 2
		radius = midLat.EarthRadius()
	} else {
		radius = loc1.Lat.EarthRadius()
	}
	distance := Meters(float64(radius) * greatCircleRadians)

	y := math.Sin(deltaLon) * c2
	x := c1*math.Sin(lat2) - math.Sin(lat1)*c2*math.Cos(deltaLon)
	bearing := math.Atan2(y, x)
	heading := HeadingF(math.Mod(toDegrees(bearing)+360, 360))
	return distance, heading
}

// Returns the location at a distance and heading from an origin, along the
// great circle arc. As above, based on
// http://www.movable-type.co.uk/scripts/latlong.html ...
//
// Formula:
//
//   φ₂ = asin( sin(φ₁)*cos(d/R) + cos(φ₁)*sin(d/R)*cos(θ) )
//   λ₂ = λ₁ + atan2( sin(θ)*sin(d/R)*cos(φ₁), cos(d/R)−sin(φ₁)*sin(φ₂) )
//
// where φ is latitude, λ is longitude, θ is the bearing (in radians,
// clockwise from north), d is the distance travelled, R is the earth’s
// radius (d/R is the angular distance, in radians; i.e. an angular distance
// of 1 is 180°).

func (origin Location) AtDistanceAndHeading(
		distance Meters, heading HeadingF) (destination Location) {
	lat1 := origin.Lat.ToRadians()
	lon1 := origin.Lon.ToRadians()
	bearing := heading.ToRadians()

	earthRadiusMeters := EstimateEarthRadiusMetersAtLatitude(origin.Lat)
	angularDistance := float64(distance / earthRadiusMeters)

	Sin := math.Sin
	Cos := math.Cos

	slat1 := math.Sin(lat1)
	clat1 := math.Cos(lat1)
	sad := math.Sin(angularDistance)
	cad := math.Cos(angularDistance)

	lat2 := math.Asin((slat1 * cad) + (clat1 * sad * Cos(bearing)))

	y := Sin(bearing) * sad * clat1
	x := cad - slat1 * Sin(lat2)
	lon2 := lon1 + math.Atan2(y, x)

	destination.Lat = Latitude(toDegrees(lat2))
	destination.Lon = Longitude(toDegrees(lon2))

	if !(-90 <= destination.Lat && destination.Lat <= 90 &&
		-180 <= destination.Lon && destination.Lon <= 180) {
		glog.Fatalf(
			"Invalid destination: %v\nOrigin: %v\nDistance: %v\nHeading: %v",
			destination, origin, distance, heading)
	}
	return
}


func (origin Location) RectCenteredAt(snDistance, weDistance Meters) Rect {
	s := origin.AtDistanceAndHeading(snDistance / 2, 180)
	n := origin.AtDistanceAndHeading(snDistance / 2, 0)
	w := origin.AtDistanceAndHeading(weDistance / 2, 270)
	e := origin.AtDistanceAndHeading(weDistance / 2, 90)
	return Rect{
		South: s.Lat,
		North: n.Lat,
		West: w.Lon,
		East: e.Lon,
	}
}