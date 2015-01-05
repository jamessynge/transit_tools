package geom

import (
	"log"
	"math"
	"reflect"
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

var _typeof_Point = reflect.TypeOf((*Point)(nil)).Elem() 





func VisitPoints(points interface{}, fn func(pt Point)) {
//Leave out optimization while debugging lower cases.
//	if s, ok := points.([]Point); ok {
//		for _, pt := range s {
//			fn(pt)
//		}
//		return
//	}

	t := reflect.TypeOf(points)
	if !(t.Kind() == reflect.Array || t.Kind() == reflect.Slice) {
		log.Fatal("Not an array or slice: ", t.String())
	}
	pv := reflect.ValueOf(points)
	l := pv.Len()

	et := t.Elem()
	if et.ConvertibleTo(_typeof_Point) {
		// Points is a slice of objects convertable to Point.
		for n := 0; n < l; n++ {
			ev := pv.Index(n)
			fn(ev.Interface().(Point))
		}
		return
	}
	if et.Kind() != reflect.Ptr {
		if et.Kind() != reflect.Struct {
			log.Fatal("Unable to convert to a geom.Point: ", et.String())
		}
		sf, ok := et.FieldByName("Point")
		if ok && !(sf.Anonymous && sf.Type == _typeof_Point) {
			log.Fatal("Point field (%v) is not embedded in ", sf, et.String())
		} else if !ok {
			log.Fatal("Not a pointer, nor struct with embedded Point: ", et.String())
		}
		// Points is a slice of objects embedding Point.
		for n := 0; n < l; n++ {
			ev := pv.Index(n)
			fv := ev.FieldByName("Point")
			fn(fv.Interface().(Point))
		}
		return
	}
	et = et.Elem()
	if et.ConvertibleTo(_typeof_Point) {
		// Points is a slice of pointers to objects convertable to Pointer.
		for n := 0; n < l; n++ {
			ev := pv.Index(n)
			if ev.IsNil() { continue }
			ev = reflect.Indirect(ev)
			fn(ev.Interface().(Point))
		}
	}

	// Is it an embedded struct field?
	if et.Kind() != reflect.Struct {
		log.Fatalf("Unable to convert to a Point: %f, in %s", et, t)
	}
	sf, ok := et.FieldByName("Point")
	if ok && !(sf.Anonymous && sf.Type == _typeof_Point) {
		log.Fatalf("Point field (%v) is not embedded in %s, in %s", sf, et, t)
	} else if !ok {
		log.Fatalf("No field Point in %s, in %s", et, t)
	}
	for n := 0; n < l; n++ {
		ev := pv.Index(n)
		if ev.IsNil() { continue }
		ev = reflect.Indirect(ev)
		fv := ev.FieldByName("Point")
		fn(fv.Interface().(Point))
	}
	return
}

func PointsBounds(points interface{}) (bounds Rect) {
	first := true
	VisitPoints(points, func(pt Point) {
		if first {
			first = false
			bounds.MinX, bounds.MinY = pt.X, pt.Y
			bounds.MaxX, bounds.MaxY = pt.X, pt.Y
			return
		}
		if pt.X < bounds.MinX {
			bounds.MinX = pt.X
		} else if pt.X > bounds.MaxX {
			bounds.MaxX = pt.X
		}
		if pt.Y < bounds.MinY {
			bounds.MinY = pt.Y
		} else if pt.Y > bounds.MaxY {
			bounds.MaxY = pt.Y
		}
	})
	return
}

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
