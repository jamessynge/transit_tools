package geogeom

import (
	"fmt"
	"geo"
	"geom"
	"math"
)

type HeadingTransform interface {
	// Heading (clockwise degrees, 0 north) to direction
	// (counter-clockwise radians, 0 horizontal).
	GeoHeadingToDirection(heading geo.Heading) (direction float64, err error)
	ToDirection(heading float64) (direction float64)
	FromDirection(direction float64) (heading float64)
}

type CoordTransform interface {
	HeadingTransform

	ToPoint(loc geo.Location) (pt geom.Point)
	FromPoint(pt geom.Point) (loc geo.Location, err error)
}

type NoOpHeadingTransform struct{}

func (t NoOpHeadingTransform) GeoHeadingToDirection(heading geo.Heading) (
	direction float64, err error) {
	if heading.IsValid() {
		direction = t.ToDirection(float64(heading))
	} else {
		err = fmt.Errorf("Invalid geo.Heading: %v", heading)
	}
	return
}
func (t NoOpHeadingTransform) ToDirection(heading float64) float64 {
	// Change direction from increasing clockwise to increasing counterclockwise.
	heading = math.Mod(90-heading, 360)
	if heading < 0 {
		// math.Mod is implemented with truncated division.
		heading = 360 + heading
	}
	return heading * math.Pi / 180
}
func (t NoOpHeadingTransform) FromDirection(direction float64) float64 {
	heading := math.Mod(90-direction*180/math.Pi, 360)
	if heading < 0 {
		// math.Mod is implemented with truncated division.
		heading = 360 + heading
	}
	return heading
}
func MakeNoOpHeadingTransform() HeadingTransform {
	return NoOpHeadingTransform{}
}

type NoOpCoordTransform struct {
	NoOpHeadingTransform
}

func (t NoOpCoordTransform) ToPoint(loc geo.Location) geom.Point {
	return geom.Point{X: float64(loc.Lon), Y: float64(loc.Lat)}
}
func (t NoOpCoordTransform) FromPoint(pt geom.Point) (geo.Location, error) {
	return geo.LocationFromFloat64s(pt.Y, pt.X)
}

////
//func (t NoOpCoordTransform) GeoHeadingToDirection(heading geo.Heading) (
//		direction float64, err error) {
//	if 0 <= heading && heading <= 360 {
//		direction = t.ToDirection(float64(heading))
//	} else {
//		err = fmt.Errorf("Invalid geo.Heading: %v", heading)
//	}
//	return
//}
//func (t NoOpCoordTransform) ToDirection(heading float64) float64 {
//	// Change direction from increasing clockwise to increasing counterclockwise.
//	heading = math.Mod(90 - heading, 360)
//	if heading < 0 {
//		// Mod is implemented with truncated division, so need to flip sign.
//		heading = -heading
//	}
//	return heading*math.Pi/180
//}
//func (t NoOpCoordTransform) FromDirection(direction float64) float64 {
//	heading := math.Mod(90 - direction*180/math.Pi, 360)
//	if heading < 0 {
//		// Mod is implemented with truncated division, so need to flip sign.
//		heading = -heading
//	}
//	return heading
//}

func MakeNoOpCoordTransform() CoordTransform {
	return NoOpCoordTransform{}
}

// Translation from Lat-Lon to a (small) flat region around a single Lat-Lon
// point (i.e. a metro area).
type MetricCoordTransform struct {
	center geo.Location

	// FOR NOW, assuming that the central latitude is close enough
	// to the equator that we can ignore distortions to headings.
	NoOpHeadingTransform

	//	earthRadiusMeters float64
}

func (p *MetricCoordTransform) ToPoint(loc geo.Location) geom.Point {
	distance, heading := geo.ToDistanceAndHeading(p.center, loc)
	direction := p.ToDirection(heading)
	x := math.Cos(direction) * distance
	y := math.Sin(direction) * distance
	return geom.Point{X: x, Y: y}
}
func (p *MetricCoordTransform) FromPoint(pt geom.Point) (loc geo.Location, err error) {
	distance := math.Hypot(pt.X, pt.Y)
	if distance > 50*1000 { // Assuming Earth
		err = fmt.Errorf("Point too far from center (%v meters)", distance)
		return
	}
	direction := math.Atan2(pt.Y, pt.X)
	heading := p.FromDirection(direction)
	loc = geo.FromDistanceAndHeading(p.center, distance, heading)
	return
}

//func (p *MetricCoordTransform) GeoHeadingToDirection(heading geo.Heading) (
//		float64, error) {
//	return p.ToDirection(float64(heading))
//}
//func (p *MetricCoordTransform) ToDirection(heading float64) float64 {
//	// Change direction from increasing clockwise to increasing counterclockwise.
//	heading = math.Mod(90 - heading, 360)
//	if heading < 0 {
//		// Mod is implemented with truncated division, so need to flip sign.
//		heading = -heading
//	}
//	return heading*math.Pi/180
//}
//func (p *MetricCoordTransform) FromDirection(direction float64) float64 {
//	heading := math.Mod(90 - direction*180/math.Pi, 360)
//	if heading < 0 {
//		// Mod is implemented with truncated division, so need to flip sign.
//		heading = -heading
//	}
//	return heading
//}

func MakeMetricCoordTransform(center geo.Location) CoordTransform {
	return &MetricCoordTransform{center: center}
}

func LocationToPoint(loc geo.Location) geom.Point {
	return geom.Point{X: float64(loc.Lon), Y: float64(loc.Lat)}
}

func LocationAndHeadingToRay(loc geo.Location, heading float64) geom.Ray {
	return geom.RayFromPtAndHeading(LocationToPoint(loc), heading)
}

func LocationsCollectionToPoints(
	numLocations int, getLocation func(index int) geo.Location,
	transform CoordTransform) []geom.Point {
	result := make([]geom.Point, numLocations)
	for i := 0; i < numLocations; i++ {
		//		from := getLocation(i)
		//		to := transform.ToPoint(from)
		//		log.Printf("Translation %d: location %v  ->  %v", i,

		result[i] = transform.ToPoint(getLocation(i))
	}
	return result
}
