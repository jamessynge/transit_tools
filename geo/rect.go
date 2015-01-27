package geo

import (
"fmt"
"math"
)

type Rect struct {
	South Latitude
	North Latitude
	West  Longitude
	East  Longitude
}
func (p *Rect) Normalize() {
	p.South = p.South.Clamp()
	p.North = p.North.Clamp()
	if p.South > p.North {
		p.North, p.South = p.South, p.North
	}

	p.West = p.West.Clamp()
	p.East = p.East.Clamp()
	if p.West > p.East {
		p.East, p.West = p.West, p.East
	}
}
// Assuming that the rectangle is not near the poles
// (at least non-nearer than margin).
func (r Rect) Expand(margin Meters) Rect {
	nw := Location{Lat: r.North, Lon: r.West}
	se := Location{Lat: r.South, Lon: r.East}

	w_of_nw := nw.AtDistanceAndHeading(margin, 270)
	n_of_nw := nw.AtDistanceAndHeading(margin, 0)

	e_of_se := se.AtDistanceAndHeading(margin, 90)
	s_of_se := se.AtDistanceAndHeading(margin, 180)

	return Rect{
		South: s_of_se.Lat,
		North: n_of_nw.Lat,
		West: w_of_nw.Lon,
		East: e_of_se.Lon,
	}
}
func (r Rect) DeltaLongitude() Longitude {
	return r.East - r.West
}
func (r Rect) LongitudeFrac() float64 {
	return float64(r.DeltaLongitude()) / 360
}
func (r Rect) DeltaLatitude() Latitude {
	return r.North - r.South
}
func (r Rect) Height() Meters {
	return r.South.ArcLength(r.North)
}
func (r Rect) WidthAtLatitude(l Latitude) Meters {
	radius := l.EarthRadius()
	latitudeCircum := float64(radius) * math.Cos(l.ToRadians())
	return Meters(latitudeCircum * r.LongitudeFrac())
}
// Width of the rectangle at the southern edge.
func (r Rect) SouthernWidth() Meters {
	return r.WidthAtLatitude(r.South)
}
// Width of the rectangle at the latitude half way between the South and North.
func (r Rect) MiddleWidth() Meters {
	return r.WidthAtLatitude((r.South - r.North) / 2)
}
// Width of the rectangle at the northern edge.
func (r Rect) NorthernWidth() Meters {
	return r.WidthAtLatitude(r.North)
}
func (r Rect) Area() MetersSq {
	stripArea := float64(r.South.AreaOfCap() - r.North.AreaOfCap())
	return MetersSq(stripArea * r.LongitudeFrac())
}
func (r Rect) Center() Location {
	return Location{
		Lat: (r.South + r.North) / 2,
		Lon: (r.West + r.East) / 2,
	}
}
func (r Rect) String() string {
	return fmt.Sprintf("geo.Rect[%s to %s, %s to %s]",
			r.West.FormatDMS(), r.East.FormatDMS(),
			r.South.FormatDMS(), r.North.FormatDMS())
}
