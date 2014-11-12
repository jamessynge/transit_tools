package nextbus

import (
	"github.com/jamessynge/transit_tools/geo"
)

type Location struct {
	geo.Location
	Stops  map[string]*Stop
	Paths  map[int]*Path // Path.Index to *Path
	Agency *Agency
}
