package geom

import (
//	"log"
)

// Represents the directed line segment from seg.Pt1 to seg.Pt2
// (i.e. direction matters).
type DirectedSegment struct {
	Segment
	Direction

	// Values used for distance to point calculations.
	dX, dY                      float64
	lengthMeters, lengthSquared float64
}

func InitDirectedSegment(p *DirectedSegment, pt1, pt2 Point) {
	p.Segment.Pt1 = pt1
	p.Segment.Pt2 = pt2
	p.dX = pt2.X - pt1.X
	p.dY = pt2.Y - pt1.Y
	p.lengthMeters = p.Length()
	p.lengthSquared = p.lengthMeters * p.lengthMeters
	p.Direction = MakeDirectionFromVector(p.dX, p.dY)
}

func NewDirectedSegment(pt1, pt2 Point) *DirectedSegment {
	p := &DirectedSegment{}
	InitDirectedSegment(p, pt1, pt2)
	//	log.Printf("NewDirectedSegment: %#v", p)
	return p
}

func MakeDirectedSegments(
	points []Point) (result []*DirectedSegment) {
	result = make([]*DirectedSegment, len(points)-1)
	for ndx := 0; ndx < len(points)-1; ndx++ {
		result[ndx] = NewDirectedSegment(points[ndx], points[ndx+1])
	}
	return
}

func (p DirectedSegment) ClosestPointTo(pt Point) (closest Point, perpendicular bool) {
	n := (pt.X-p.Pt1.X)*p.dX +
		(pt.Y-p.Pt1.Y)*p.dY
	r := n / p.lengthSquared
	if r < 0 {
		return p.Pt1, false
	}
	if r > 1 {
		return p.Pt2, false
	}
	x := p.Pt1.X + r*p.dX
	y := p.Pt1.Y + r*p.dY
	return Point{x, y}, true
}

func (p DirectedSegment) DistanceToPoint(pt Point) float64 {
	closestPt, _ := p.ClosestPointTo(pt)
	return pt.Distance(closestPt)
}

func (p *DirectedSegment) AngleBetween(direction Direction) (angle float64, ok bool) {
	if !direction.DirectionIsValid() {
		return
	}
	angle = direction.AngleBetween(direction)
	ok = true
	return
}
