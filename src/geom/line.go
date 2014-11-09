package geom

import (
	"log"
	"math"
)

type Line interface {
	// Angle of line relative to horizontal, in radians (i.e. 0 == horizontal).
	// Range: [0,pi)
	Angle() float64

	// Distance from the point pt to the line.
	//	Distance(pt Point) float64

	// Returns the point on the line that is nearest to the point pt.
	NearestPointTo(pt Point) Point

	// Returns the point where the two lines intersect, and a bool indicating
	// whether or not the two lines do intersect.
	Intersection(ln Line) (pt Point, ok bool)
}

const (
	lowerVerticalLimit = northDirection - math.SmallestNonzeroFloat32
	upperVerticalLimit = northDirection + math.SmallestNonzeroFloat32

	lowerHorizontalLimit = eastDirection + math.SmallestNonzeroFloat32
	upperHorizontalLimit = westDirection - math.SmallestNonzeroFloat32
)

func IsVerticalLine(line Line) bool {
	angle := line.Angle()
	return lowerVerticalLimit <= angle && angle <= upperVerticalLimit
}

func IsHorizontalLine(line Line) bool {
	angle := line.Angle()
	return angle <= lowerHorizontalLimit || upperHorizontalLimit <= angle
}

type TwoPointLine struct {
	pt1, pt2      Point
	angle, length float64 // Optimizations, not essential.
	// Could add lengthSquared, dx and dy which are used in NearestPointTo, if
	// that is called a lot (which it may be).  Alternately, all these
	// optimizations could be moved into a LineMatcher, built from a Line or
	// a TwoPointLine, and move the NearestPointTo operation to that matcher.
}

// pt1 and pt2 must be distinct else NearestPointTo will divide by zero.
func LineFromTwoPoints(pt1, pt2 Point) *TwoPointLine {
	ln := &TwoPointLine{pt1: pt1, pt2: pt2}
	dx := pt2.X - pt1.X
	dy := pt2.Y - pt1.Y
	ln.length = math.Hypot(dx, dy)
	ln.angle = math.Atan2(dy, dx)
	if ln.angle < 0 {
		ln.angle += twoPi
	}
	ln.angle = math.Mod(ln.angle, math.Pi)
	return ln
}

func (ln *TwoPointLine) Angle() float64 {
	return ln.angle
}

// The nearest point is the intersection of line
// and a perpendicular line through point pt.
func (ln *TwoPointLine) NearestPointTo(pt Point) Point {
	/*
		if IsHorizontalLine(ln) {
			// Line is horizontal, or very nearly so (y is constant).
			return Point{pt.X, ln.Y}
		}
		if IsVerticalLine(ln) {
			// Line is vertical, or very nearly so (x is constant).
			return Point{ln.X, pt.Y}
		}
	*/
	dx := ln.pt2.X - ln.pt1.X
	dy := ln.pt2.Y - ln.pt1.Y

	l2 := ln.length * ln.length
	n := ((pt.X-ln.pt1.X)*dx +
		(pt.Y-ln.pt1.Y)*dy)
	r := n / l2

	// r is how far the projection of pt on to the line is along the segment from
	// ln.pt1 (0) to ln.pt2 (1); might be negative or greater than 1.

	x := ln.pt1.X + r*dx
	y := ln.pt1.Y + r*dy
	return Point{x, y}
}

// ok == true if able to compute a single point of intersection.
// false if the lines are parallel, or if ln is not a TwoPointLine.
func (line1 *TwoPointLine) Intersection(ln Line) (pt Point, ok bool) {
	line2, ok := ln.(*TwoPointLine)
	if !ok {
		log.Panicf("Unsupported Line implementation: %T", ln)
		return
	}

	if line1.angle == line2.angle {
		ok = false // Parallel or coincident, so no single intersection.
		return
	}

	// From http://en.wikipedia.org/wiki/Line-line_intersection
	x1, y1 := line1.pt1.X, line1.pt1.Y
	x2, y2 := line1.pt2.X, line1.pt2.Y
	x3, y3 := line2.pt1.X, line2.pt1.Y
	x4, y4 := line2.pt2.X, line2.pt2.Y

	denom := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)

	pt.X = ((x1*y2-y1*x2)*(x3-x4) - (x1-x2)*(x3*y4-y3*x4)) / denom
	pt.Y = ((x1*y2-y1*x2)*(y3-y4) - (y1-y2)*(x3*y4-y3*x4)) / denom
	return pt, true
}

/*
type PointAngleLine struct {
	Point
	angle float64 // Radians; 0 is to the right, pi/2 is up. Range: [0, pi)
}

// headingDegrees: 0 is north (up on a map), 90 is east (right on a map);
// converts to pi/2 and 0, respectively.  0 and 180 both map to pi/2, as this
// is a line, and not a ray.
func InitFromXYHeading(x, y float64, headingDegrees int) (pal PointAngleLine) {
	pal.angle = float64((-headingDegrees+90)%180) * math.Pi / 180.0
	pal.Point.X = x
	pal.Point.Y = y
	return
}

// headingDegrees: 0 is north (up on a map), 90 is east (right on a map);
// converts to pi/2 and 0, respectively.  0 and 180 both map to pi/2, as this
// is a line, and not a ray.
func LineFromXYAngle(x, y, angle float64) (pal PointAngleLine) {
	pal.angle = angle
	pal.Point.X = x
	pal.Point.Y = y
	return
}

func (pt PointAngleLine) Angle() float64 {
	return pt.angle
}

// The nearest point is the intersection of line
// and a perpendicular line through point pt.
func (line PointAngleLine) NearestPointTo(pt Point) Point {
	if line.angle <= math.SmallestNonzeroFloat32 {
		// Line is horizontal, or very nearly so (y is constant).
		return Point{pt.X, line.Y}
	}
	if math.Abs(line.angle-northDirection) <= math.SmallestNonzeroFloat32 {
		// Line is vertical, or very nearly so (x is constant).
		return Point{line.X, pt.Y}
	}
	line2 := PointAngleLine{pt, math.Mod(line.angle+halfPi, math.Pi)}

	pt2, _ := LinesIntersection(line, line2)
	return pt2
}

// ok == false iff the lines are parallel.
func LinesIntersection(line1, line2 PointAngleLine) (pt Point, ok bool) {

	return
}
*/
