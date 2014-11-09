package geom

import (
	"math"
)

type Point struct {
	X, Y float64
}

func (p Point) InsideRect(r Rect) bool {
	return r.MinX <= p.X && r.MinY <= p.Y &&
		r.MaxX >= p.X && r.MaxY >= p.Y
}

func (p Point) Minus(o Point) Point {
	return Point{p.X - o.X, p.Y - o.Y}
}

func (p Point) DotProduct(o Point) float64 {
	return p.X*o.X + p.Y*o.Y
}

func (p Point) Length() float64 {
	return math.Hypot(p.X, p.Y)
}

func (p Point) Distance(o Point) float64 {
	return math.Hypot(p.X-o.X, p.Y-o.Y)
}

func (p Point) NearlyEqual(o Point) bool {
	return p.Distance(o) <= 0.00001
}

func (p Point) ToRect(xBorder, yBorder float64) Rect {
	return NewRect(p.X-xBorder, p.X+xBorder, p.Y-yBorder, p.Y+yBorder)
}

func (p Point) DirectionTo(o Point) float64 {
	dx := o.X - p.X
	dy := o.Y - p.Y
	direction := math.Atan2(dy, dx)
	if direction < 0 {
		direction += (2 * math.Pi)
	}
	return direction
}

/*
func (p Point) Scale(s float64) Point {
	return Point{p.X * s, p.Y * s}
}

func (p Point) Normalize() Point {
	l := p.Length()
	if l <= 0 {
		panic(fmt.Errorf("Unable to normalize this Point (vector): %#v", p))
	}
	return p.Scale(1 / l)
}
*/

// Implements stats.Data2DSource with weight == 1 always
type PointSlice []Point

func (ps PointSlice) Len() int {
	return len(ps)
}
func (ps PointSlice) X(n int) float64 {
	return ps[n].X
}
func (ps PointSlice) Y(n int) float64 {
	return ps[n].Y
}
func (ps PointSlice) Weight(n int) float64 {
	return 1
}

/*

type UnweightedPointsSource struct {
	Points []Point
}
func (p *UnweightedPointsSource) Len() int {
	return len(p.Points)
}
func (p *UnweightedPointsSource) X(n int) float64 {
	return p.Points[n].X
}
func (p *UnweightedPointsSource) Y(n int) float64 {
	return p.Points[n].Y
}
func (p *UnweightedPointsSource) Weight(n int) float64 {
	return 1
}
*/
