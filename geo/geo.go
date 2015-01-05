package geo

import (
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/jamessynge/transit_tools/util"
)

type Latitude float64
type Longitude float64

// Valid headings are in the range 0 to 360 (inclusive, for some reason of
// 360; perhaps rounding errors?).  Negative values indicate that the heading
// is unavailable; usually I see -1 in that case, but occasionally see -2.
// 0 and 360 appear to be north, 90 east, 180 south, 270 west.
type Heading int

type Location struct {
	Lat Latitude
	Lon Longitude
}

func (u Location) String() string {
	return fmt.Sprintf("(%f, %f)", u.Lat, u.Lon)
}

func (u Location) SameLocation(v Location) bool {
	return u.Lat == v.Lat && u.Lon == v.Lon
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
func ParseLatitude(s string) (lat Latitude, err error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Latitude(math.NaN()), err
	}
	return LatitudeFromFloat64(v)
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
func ParseLongitude(s string) (lon Longitude, err error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Longitude(math.NaN()), err
	}
	return LongitudeFromFloat64(v)
}
func LocationFromFloat64s(lat64, lon64 float64) (loc Location, err error) {
	loc.Lat, err = LatitudeFromFloat64(lat64)
	if err == nil {
		loc.Lon, err = LongitudeFromFloat64(lon64)
	}
	return loc, err
}

// Format as degrees, minutes and seconds, with a suffix for sign.
func formatDMS(v float64, neg, pos string) string {
	var suffix string
	if v < 0 {
		suffix = neg
		v = -v
	} else {
		suffix = pos
	}
	deg := int(v)
	v -= float64(deg)
	v *= 60
	min := int(v)
	v -= float64(min)
	v *= 60
	return fmt.Sprintf("%d.%02d.%04.1f%s", deg, min, v, suffix)
}

func (v Latitude) FormatDMS() string {
	return formatDMS(float64(v), "S", "N")
}
func (v Longitude) FormatDMS() string {
	return formatDMS(float64(v), "W", "E")
}

//var headingPopulation int
//var headingCensus [361]int
//		headingCensus[v]++
//		headingPopulation++
//		if headingPopulation > 500000 {
//			pop := float64(headingPopulation)
//			headingPopulation = 0
//			for i, c := range headingCensus {
//				frac := float64(c) / pop * 361
////				pct := float64(c) / pop * 100
//				log.Printf("HEADING CENSUS: %ddegrees = %v fraction of expected", i, frac)
//				headingCensus[i] = 0
//			}
//		}

const (
	minHeading = -2
	maxHeading = 360
)

func HeadingFromInt(v int) (heading Heading, err error) {
	if minHeading <= v && v <= maxHeading {
		return Heading(v), nil
	}
	return Heading(-1), fmt.Errorf("Heading out of expected range: %d", v)
}

func ParseHeading(s string) (heading Heading, err error) {
	v, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		return Heading(-1), err
	}
	if minHeading <= v && v <= maxHeading {
		return Heading(int(v)), nil
	}
	return Heading(-1), fmt.Errorf("Heading out of expected range: %d", v)
}

func (h Heading) IsValid() bool {
	return 0 <= h && h <= maxHeading
}

func toRadians(degrees float64) float64 {
	return degrees * math.Pi / 180.0
}
func toDegrees(radians float64) float64 {
	return radians * 180.0 / math.Pi
}

func EstimateEarthRadiusMetersAtLatitude(lat Latitude) (meters float64) {
	radians := toRadians(float64(lat))
	// 6378137 is the earth's equatorial radius (meters).  The radius at the
	// poles is about 21384.7 meters smaller than the equatorial radius.
	meters = 6378137 - 21384.7*math.Sin(radians)
	return
}

// A common estimate of average radius of earth, in meters.
const kEarthRadiusMeters = 6371009

// Uses Haversine formula to compute the great circle distance between
// loc1 and loc2, and the initial heading at loc1 of the great circle
// path to loc2. The distance is in meters, and the heading is in degrees,
// with north = 0, east = 90.
// From: "Virtues of the Haversine", R. W. Sinnott,
//       Sky and Telescope, vol 68, no 2, 1984
// Via: http://www.movable-type.co.uk/scripts/latlong.html
func ToDistanceAndHeading(loc1, loc2 Location) (distance, heading float64) {
	deltaLat := toRadians(float64(loc2.Lat - loc1.Lat))
	deltaLon := toRadians(float64(loc2.Lon - loc1.Lon))
	lat1 := toRadians(float64(loc1.Lat))
	lat2 := toRadians(float64(loc2.Lat))

	u := math.Sin(deltaLat / 2)
	v := math.Sin(deltaLon / 2)
	c1 := math.Cos(lat1)
	c2 := math.Cos(lat2)

	a := u*u + v*v*c1*c2
	greatCircleRadians := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance = kEarthRadiusMeters * greatCircleRadians
	//	distance = EstimateEarthRadiusMetersAtLatitude(loc1.Lat) * greatCircleRadians

	y := math.Sin(deltaLon) * c2
	x := c1*math.Sin(lat2) - math.Sin(lat1)*c2*math.Cos(deltaLon)

	brng := math.Atan2(y, x)

	heading = math.Mod(toDegrees(brng)+360, 360)
	return
}

// Also from http://www.movable-type.co.uk/scripts/latlong.html ...
// Destination point given distance (meters) and heading (degrees) from a
// start point (latitude and longitude). Given a start point, initial
// heading, and distance, this will calculate the destination point and
// final bearing travelling along a (shortest distance) great circle arc.
// Formula:
//
//   φ2 = asin( sin(φ1)*cos(d/R) + cos(φ1)*sin(d/R)*cos(θ) )
//   λ2 = λ1 + atan2( sin(θ)*sin(d/R)*cos(φ1), cos(d/R)−sin(φ1)*sin(φ2) )
//
// where φ is latitude, λ is longitude, θ is the bearing (in radians,
// clockwise from north), d is the distance travelled, R is the earth’s
// radius (d/R is the angular distance, in radians)
func FromDistanceAndHeading(
	origin Location, distance, heading float64) (destination Location) {
	lat1 := toRadians(float64(origin.Lat))
	lon1 := toRadians(float64(origin.Lon))
	brng := toRadians(float64(heading))

	earthRadiusMeters := EstimateEarthRadiusMetersAtLatitude(origin.Lat)
	angularDistance := distance / earthRadiusMeters
	angularDistance = distance / kEarthRadiusMeters

	Sin := math.Sin
	Cos := math.Cos

	lat2 := math.Asin(Sin(lat1)*Cos(angularDistance) +
		Cos(lat1)*Sin(angularDistance)*Cos(brng))
	lon2 := lon1 + math.Atan2(Sin(brng)*Sin(angularDistance)*Cos(lat1),
		Cos(angularDistance)-Sin(lat1)*Sin(lat2))

	destination.Lat = Latitude(toDegrees(lat2))
	destination.Lon = Longitude(toDegrees(lon2))

	if !(-90 <= destination.Lat && destination.Lat <= 90 &&
		-180 <= destination.Lon && destination.Lon <= 180) {
		log.Panicf(
			"Invalid destination: %v\nOrigin: %v\nDistance: %v\nHeading: %v",
			destination, origin, distance, heading)
	}
	return
}

func MeasureCentralAxes(
	loLon, hiLon Longitude, loLat, hiLat Latitude) (
	snDistance, weDistance float64) {
	var south, north, west, east Location
	south.Lat = loLat
	north.Lat = hiLat
	west.Lon = loLon
	east.Lon = hiLon

	south.Lon = (loLon + hiLon) / 2
	north.Lon = south.Lon
	west.Lat = (loLat + hiLat) / 2
	east.Lat = west.Lat

	//	glog.Infof("south: %s", south)
	//	glog.Infof("north: %s", north)
	//	glog.Infof("west: %s", west)
	//	glog.Infof("east: %s", east)

	// Measure the distance.
	snDistance, _ = ToDistanceAndHeading(south, north)
	weDistance, _ = ToDistanceAndHeading(west, east)

	//	glog.Infof("snDistance: %v", snDistance)
	//	glog.Infof("weDistance: %v", weDistance)

	return
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
