package geom

import (
	"log"
	"math"
)

const (
	halfPi         = math.Pi / 2
	pi             = math.Pi
	twoPi          = math.Pi * 2
	northDirection = math.Pi / 2
	southDirection = math.Pi + math.Pi/2
	eastDirection  = 0
	westDirection  = math.Pi
)

var (
	kNegativeInfinity = math.Inf(-1)
)

type Direction struct {
	// Radians, counter clockwise, 0 to right.
	direction float64

	// dX,dY represent a unit vector in the direction the bus was moving.
	unitX, unitY float64
}

func (p Direction) CosineSimilarity(q Direction) float64 {
	return p.unitX*q.unitX + p.unitY*q.unitY
}
func (p Direction) AngleBetween(q Direction) float64 {
	if p.DirectionIsValid() && q.DirectionIsValid() {
		angle := math.Abs(p.direction - q.direction)
		if angle > pi {
			angle = twoPi - angle
		}
		return angle
	}
	return kNegativeInfinity
}
func (p Direction) DirectionIsValid() bool {
	return p.direction >= 0
}
func (p Direction) Direction() float64 {
	return p.direction
}
func (p Direction) UnitVector() (x, y float64) {
	return p.unitX, p.unitY
}
func (p Direction) UnitX() float64 {
	return p.unitX
}
func (p Direction) UnitY() float64 {
	return p.unitY
}

func MakeDirection(radians float64) (d Direction) {
	d.direction = NormalizeRadians(radians)
	d.unitX = math.Cos(d.direction)
	d.unitY = math.Sin(d.direction)
	return
}
func MakeDirectionFromVector(dX, dY float64) (d Direction) {
	distance := math.Hypot(dX, dY)
	if distance <= 0 {
		d.direction = kNegativeInfinity
	} else {
		d.unitX = dX / distance
		d.unitY = dY / distance
		d.direction = NormalizeRadians(math.Atan2(d.unitY, d.unitX))
	}
	return
}
func InvalidDirection() Direction {
	return Direction{direction: kNegativeInfinity}
}

func ToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180.0
}
func ToDegrees(radians float64) float64 {
	return radians * 180.0 / math.Pi
}
func NormalizeRadians(input float64) (output float64) {
	output = math.Mod(input, twoPi)
	if output < 0 {
		// math.Mod is implemented with truncated division (i.e. mod(-episilon, X)
		// is -episilon, not X - epsilon, which I desire for an angle).
		output = twoPi + output
	}
	if input != output {
		log.Printf("NormalizeRadians(%.3f) ==> %.3f", input, output)
	}
	return
}
