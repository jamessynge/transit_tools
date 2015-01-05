package geom

import (
	"fmt"
)

type Rect struct {
	MinX, MaxX, MinY, MaxY float64
}

func NewRect(xa, xb, ya, yb float64) Rect {
	if xa > xb {
		xa, xb = xb, xa
	}
	if ya > yb {
		ya, yb = yb, ya
	}
	return Rect{xa, xb, ya, yb}
}

func NewRectWithBorder(xa, xb, ya, yb, xborder, yborder float64) Rect {
	r := NewRect(xa, xb, ya, yb)
	r.MinX -= xborder
	r.MaxX += xborder
	r.MinY -= yborder
	r.MaxY += yborder
	return r
}

func (r Rect) Width() float64 {
	return r.MaxX - r.MinX
}

func (r Rect) Height() float64 {
	return r.MaxY - r.MinY
}

func (r Rect) Area() float64 {
	return r.Width() * r.Height()
}

func (r Rect) Intersects(o Rect) bool {
	return r.MinX < o.MaxX && r.MinY < o.MaxY &&
		r.MaxX > o.MinX && r.MaxY > o.MinY
}

func (r *Rect) IntersectsP(o *Rect) bool {
	return r.MinX < o.MaxX && r.MinY < o.MaxY &&
		r.MaxX > o.MinX && r.MaxY > o.MinY
}

func (r Rect) Contains(o Rect) bool {
	return r.MinX <= o.MinX && r.MinY <= o.MinY &&
		r.MaxX >= o.MaxX && r.MaxY >= o.MaxY
}

func (r Rect) IsPoint() bool {
	return r.MinX == r.MaxX && r.MinY == r.MaxY
}

func (r Rect) ContainsPoint(p Point) bool {
	if r.IsPoint() {
		return r.MinX == p.X && r.MinY == p.Y
	}
	return r.MinX <= p.X && p.X < r.MaxX &&
		r.MinY <= p.Y && p.Y < r.MaxY
}

func (r Rect) Center() Point {
	x := (r.MaxX-r.MinX)/2 + r.MinX
	y := (r.MaxY-r.MinY)/2 + r.MinY
	return Point{x, y}
}

// Produces the smallest axis-aligned rectangle rectangle that includes both
// r and o.
func (r Rect) Union(o Rect) Rect {
	if o.MinX > r.MinX {
		o.MinX = r.MinX
	}
	if o.MinY > r.MinY {
		o.MinY = r.MinY
	}
	if o.MaxX < r.MaxX {
		o.MaxX = r.MaxX
	}
	if o.MaxY < r.MaxY {
		o.MaxY = r.MaxY
	}
	return o
}

// Produces a new rectangle that is larger by the amount specified.
func (r Rect) AddBorder(xborder, yborder float64) Rect {
	r.MinX -= xborder
	r.MaxX += xborder
	r.MinY -= yborder
	r.MaxY += yborder
	return r
}

// Cohen-Sutherland clipping algorithm support
const (
	Inside = 0         // 0000
	Left   = 1 << iota // 0001
	Right              // 0010
	Bottom             // 0100
	Top                // 1000
)

func (r Rect) OutCode(p Point) (outCode int) {
	if p.X < r.MinX {
		outCode |= Left
	} else if p.X > r.MaxX {
		outCode |= Right
	}
	if p.Y < r.MinY {
		outCode |= Bottom
	} else if p.Y > r.MaxY {
		outCode |= Top
	}
	return
}

func (r Rect) String() string {
	return fmt.Sprintf("{X=(%v to %v), Y=(%v to %v)}", r.MinX, r.MaxX, r.MinY, r.MaxY)
}
