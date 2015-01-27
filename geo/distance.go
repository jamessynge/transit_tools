package geo

type Meters float64
type MetersSq float64

func EstimateEarthRadiusMetersAtLatitude(lat Latitude) Meters {
	return lat.EarthRadius()
}

// A common estimate of average radius of earth, in meters.
//const kEarthRadiusMeters Meters = 6371009

// Uses Haversine formula to compute the great circle distance between
// loc1 and loc2, and the initial heading at loc1 of the great circle
// path to loc2. The distance is in meters, and the heading is in degrees,
// with north = 0, east = 90.
// From: "Virtues of the Haversine", R. W. Sinnott,
//       Sky and Telescope, vol 68, no 2, 1984
// Via: http://www.movable-type.co.uk/scripts/latlong.html
func ToDistanceAndHeading(loc1, loc2 Location) (distance, heading float64) {
	d, h := loc1.DistanceAndHeadingTo(loc2)
	return float64(d), float64(h)
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
	return origin.AtDistanceAndHeading(Meters(distance), HeadingF(heading))
}
