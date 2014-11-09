package geom

import (
	"math"
)

type Ray interface {
	Start() Point

	// Radians; 0 is to the right, pi/2 is up. Range: [0, 2*pi)
	Direction() float64
}

type ray struct {
	start     Point
	direction float64
}

func (p *ray) Start() Point {
	return p.start
}
func (p *ray) Direction() float64 {
	return p.direction
}

// Heading is in degrees, 0 is north (up), 90 is east (right).
func RayFromPtAndHeading(pt Point, heading float64) Ray {
	// Change direction from increasing clockwise to increasing counterclockwise.
	heading = math.Mod(90-heading, 360)
	if heading < 0 {
		// Mod is implemented with truncated division, so need to flip sign.
		heading = -heading
	}
	return &ray{start: pt, direction: heading * math.Pi / 180}
}
