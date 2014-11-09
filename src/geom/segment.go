package geom

import (
	"math"
)

type Segment struct {
	Pt1, Pt2 Point
}

func (s Segment) Inside(r Rect) bool {
	return r.ContainsPoint(s.Pt1) && r.ContainsPoint(s.Pt2)
}
func (s Segment) Bounds() Rect {
	return NewRect(s.Pt1.X, s.Pt2.X, s.Pt1.Y, s.Pt2.Y)
}
func (s Segment) Length() float64 {
	return math.Hypot(s.Pt1.X-s.Pt2.X, s.Pt1.Y-s.Pt2.Y)
}

// Direction from Pt1 to Pt2 (counter-clockwise radians, 0 to right).
func (s Segment) Direction() float64 {
	direction := math.Atan2(s.Pt2.Y-s.Pt1.Y, s.Pt2.X-s.Pt1.X)
	if direction < 0 {
		direction += math.Pi
	}
	return direction
}
func (s Segment) NearlyEqual(o Segment) bool {
	return s.Pt1.NearlyEqual(o.Pt1) && s.Pt2.NearlyEqual(o.Pt2)
}

// Will typically be called when segment cross some edge of r.
func (s Segment) IntersectBounds(r Rect) (intersection Rect, empty bool) {
	clipped, doesIntersect := s.Clip(r)
	if doesIntersect {
		intersection = NewRect(clipped.Pt1.X, clipped.Pt2.X, clipped.Pt1.Y, clipped.Pt2.Y)
	} else {
		empty = true
	}
	return
}

// TODO Consider specializing Clip for Intersects
func (s Segment) Intersects(r Rect) bool {
	_, doesIntersect := s.Clip(r)
	return doesIntersect
}

// Will typically be called when segment cross some edge of r.
func (s Segment) Clip(r Rect) (clipped Segment, doesIntersect bool) {
	// Cohen-Sutherland clipping algorithm
	pt0, pt1 := s.Pt1, s.Pt2
	outCode0 := r.OutCode(pt0)
	outCode1 := r.OutCode(pt1)
	for {
		if (outCode0 | outCode1) == 0 {
			// Bitwise OR is 0. Trivially accept and get out of loop
			doesIntersect = true
			break
		}
		if (outCode0 & outCode1) != 0 {
			// Bitwise AND is not 0. Trivially reject and get out of loop
			break
		}
		// failed both tests, so calculate the line segment to clip
		// from an outside point to an intersection with clip edge
		var p Point
		// At least one endpoint is outside the clip rectangle; pick it.
		outCodeOut := outCode0
		if outCodeOut == 0 {
			outCodeOut = outCode1
		}
		// Now find the intersection point; use formulas:
		//    y = pt0.Y + slope * (x - pt0.X),
		//    x = pt0.X + (1 / slope) * (y - pt0.Y)
		if (outCodeOut & Top) != 0 {
			// point is above the clip rectangle
			p.X = pt0.X + (pt1.X-pt0.X)*(r.MaxY-pt0.Y)/(pt1.Y-pt0.Y)
			p.Y = r.MaxY
		} else if (outCodeOut & Bottom) != 0 {
			// point is below the clip rectangle
			p.X = pt0.X + (pt1.X-pt0.X)*(r.MinY-pt0.Y)/(pt1.Y-pt0.Y)
			p.Y = r.MinY
		} else if (outCodeOut & Right) != 0 {
			// point is to the right of clip rectangle
			p.Y = pt0.Y + (pt1.Y-pt0.Y)*(r.MaxX-pt0.X)/(pt1.X-pt0.X)
			p.X = r.MaxX
		} else if (outCodeOut & Left) != 0 {
			// point is to the left of clip rectangle
			p.Y = pt0.Y + (pt1.Y-pt0.Y)*(r.MinX-pt0.X)/(pt1.X-pt0.X)
			p.X = r.MinX
		}
		// Now we move outside point to intersection point to clip
		// and get ready for next pass.
		if outCodeOut == outCode0 {
			pt0 = p
			outCode0 = r.OutCode(pt0)
		} else {
			pt1 = p
			outCode1 = r.OutCode(pt1)
		}
	}
	if doesIntersect {
		clipped.Pt1 = pt0
		clipped.Pt2 = pt1
	}
	return
}
