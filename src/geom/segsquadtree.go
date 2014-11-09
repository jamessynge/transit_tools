package geom

import (
	"fmt"
	"math"
)

type SQTreeDatum interface {
	Segment() Segment
	Data() interface{}
	Bounds() Rect
	Length() float64
	DistToPoint(p Point) float64
}

type sqtSeg struct {
	next   *sqtSeg
	seg    Segment
	bounds Rect
	//	unitVec		Point  // Unit vector from seg.Pt1 toward seg.Pt2
	length float64 // Length of seg
	data   interface{}
}

func newSQTSeg(seg Segment, data interface{}) *sqtSeg {
	return &sqtSeg{
		seg:    seg,
		bounds: seg.Bounds(),
		length: seg.Length(),
		data:   data,
	}
}

func (s *sqtSeg) Segment() Segment {
	return s.seg
}

func (s *sqtSeg) Data() interface{} {
	return s.data
}

func (s *sqtSeg) Bounds() Rect {
	return s.bounds
}

func (s *sqtSeg) Length() float64 {
	return s.length
}

func (s *sqtSeg) DistToPoint(p Point) float64 {
	l2 := s.length * s.length
	n := ((p.X-s.seg.Pt1.X)*(s.seg.Pt2.X-s.seg.Pt1.X) +
		(p.Y-s.seg.Pt1.Y)*(s.seg.Pt2.Y-s.seg.Pt1.Y))
	r := n / l2
	if r < 0 {
		return p.Distance(s.seg.Pt1)
	}
	if r > 1 {
		return p.Distance(s.seg.Pt2)
	}

	x := s.seg.Pt1.X + r*(s.seg.Pt2.X-s.seg.Pt1.X)
	y := s.seg.Pt1.Y + r*(s.seg.Pt2.Y-s.seg.Pt1.Y)
	return p.Distance(Point{x, y})
}

// A root, interior or leaf element in a segments quadtree.
type sqtnode struct {
	bounds  Rect
	center  Point
	level   int
	headSeg *sqtSeg
	kids    [4]*sqtnode
}

// Use of each of the kids
const (
	kUpperLeft = iota
	kUpperRight
	kLowerLeft
	kLowerRight
)

// Caller has ensured that the segment is inside the bounds of this node.
func (t *sqtnode) insertSegment(newSeg *sqtSeg) (depth int) {
	// Does the segment belong in a child node?
	if newSeg.bounds.MaxX <= t.center.X {
		if newSeg.bounds.MaxY <= t.center.Y {
			return t.insertSegmentInQuadrant(kLowerLeft, newSeg)
		} else if newSeg.bounds.MinY >= t.center.Y {
			return t.insertSegmentInQuadrant(kUpperLeft, newSeg)
		}
	}
	if newSeg.bounds.MinX >= t.center.X {
		if newSeg.bounds.MaxY <= t.center.Y {
			return t.insertSegmentInQuadrant(kLowerRight, newSeg)
		} else if newSeg.bounds.MinY >= t.center.Y {
			return t.insertSegmentInQuadrant(kUpperRight, newSeg)
		}
	}
	newSeg.next = t.headSeg
	t.headSeg = newSeg
	return t.level
}

// Caller has ensured that the segment is inside the bounds of this node,
// and inside the bounds of the specified quadrant.
func (t *sqtnode) insertSegmentInQuadrant(quadrant int, newSeg *sqtSeg) (depth int) {
	kid := t.kids[quadrant]
	if kid == nil {
		kid = &sqtnode{bounds: t.bounds, level: t.level + 1}
		switch quadrant {
		case kUpperLeft:
			kid.bounds.MaxX = t.center.X
			kid.bounds.MinY = t.center.Y
		case kUpperRight:
			kid.bounds.MinX = t.center.X
			kid.bounds.MinY = t.center.Y
		case kLowerLeft:
			kid.bounds.MaxX = t.center.X
			kid.bounds.MaxY = t.center.Y
		case kLowerRight:
			kid.bounds.MinX = t.center.X
			kid.bounds.MaxY = t.center.Y
		default:
			panic(fmt.Errorf("invalid quadrant %v", quadrant))
		}
		kid.center = kid.bounds.Center()
		t.kids[quadrant] = kid
	}
	return kid.insertSegment(newSeg)
}

// Caller has confirmed that bounds intersects t.bounds.
func (t *sqtnode) visit(bounds Rect, accepter Accepter) {
	for s := t.headSeg; s != nil; s = s.next {
		if bounds.Intersects(s.bounds) {
			accepter.Accept(s)
		}
	}
	for _, kid := range t.kids {
		if kid != nil && kid.bounds.Intersects(bounds) {
			kid.visit(bounds, accepter)
		}
	}
}

func NewSQTNode(bounds Rect) *sqtnode {
	return &sqtnode{bounds: bounds, center: bounds.Center()}
}

type SQTree struct {
	root sqtnode
}

func NewSQTree(bounds Rect) *SQTree {
	return &SQTree{*NewSQTNode(bounds)}
}

func (t *SQTree) InsertSegment(seg Segment, data interface{}) (depth int, err error) {
	depth = 0
	newSeg := newSQTSeg(seg, data)
	if !t.root.bounds.Contains(newSeg.bounds) {
		err = fmt.Errorf("Segment (%#v) not inside bounds (%#v)", seg, t.root.bounds)
	} else {
		depth = t.root.insertSegment(newSeg)
	}
	return
}

type Accepter interface {
	Accept(datum SQTreeDatum)
}

func (t *SQTree) Visit(bounds Rect, accepter Accepter) {
	if t.root.bounds.Intersects(bounds) {
		t.root.visit(bounds, accepter)
	}
}

type chanAccepter struct {
	c chan<- SQTreeDatum
}

func (c *chanAccepter) Accept(datum SQTreeDatum) {
	c.c <- datum
}

func (t *SQTree) Send(bounds Rect, c chan<- SQTreeDatum) {
	accepter := &chanAccepter{c}
	t.Visit(bounds, accepter)
	close(c)
}

func (t *SQTree) NearestSegment(pt Point, maxDistance float64) (
	seg Segment, data interface{}, distance float64) {
	fmt.Printf("NearestSegment: pt=%v, maxDistance=%v\n", pt, maxDistance)
	bounds := NewRectWithBorder(pt.X, pt.X, pt.Y, pt.Y, maxDistance, maxDistance)
	closestDistance := math.MaxFloat64
	var closestDatum SQTreeDatum
	c := make(chan SQTreeDatum)

	go t.Send(bounds, c)
	for {
		datum, ok := <-c
		if !ok {
			break
		}
		dist := datum.DistToPoint(pt)
		if dist > maxDistance {
			continue
		}
		if dist < closestDistance {
			closestDistance = dist
			closestDatum = datum
		}
	}

	if closestDatum != nil {
		return closestDatum.Segment(), closestDatum.Data(), closestDistance
	}
	return Segment{}, nil, closestDistance
}
