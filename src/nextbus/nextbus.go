package nextbus

import (
	"geo"
)

type Location struct {
	geo.Location
	Stops  map[string]*Stop
	Paths  map[int]*Path // Path.Index to *Path
	Agency *Agency
}
