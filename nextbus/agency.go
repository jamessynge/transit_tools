package nextbus

import (
	"fmt"
	"github.com/jamessynge/transit_tools/geo"
	"hash"
	"hash/fnv"
	"log"
	"math"
)

type Agency struct {
	Tag    string
	Routes map[string]*Route
	// Because a vehicleLocation report sometimes erroneously has mismatched
	// routeTag and dirTag, we have an agency level index of Directions.
	Directions    map[string]*Direction
	Stops         map[string]*Stop
	PathsByHash   map[uint64][]*Path // Map key is a hash, which my have collisions
	PathsByIndex  map[int]*Path
	lastPathIndex int
	Locations     map[geo.Location]*Location
}

func NewAgency(tag string) *Agency {
	return &Agency{
		Tag:          tag,
		Routes:       make(map[string]*Route),
		Directions:   make(map[string]*Direction),
		Stops:        make(map[string]*Stop),
		PathsByHash:  make(map[uint64][]*Path),
		PathsByIndex: make(map[int]*Path),
		Locations:    make(map[geo.Location]*Location),
	}
}
func (p *Agency) NumPaths() int {
	return p.lastPathIndex
}
func (p *Agency) GetPaths() []*Path {
	var result []*Path
	for _, path := range p.PathsByIndex {
		result = append(result, path)
	}
	return result
}
func (p *Agency) GetPath(index int) *Path {
	return p.PathsByIndex[index]
}

func (p *Agency) GetPathBounds() (min, max geo.Location, ok bool) {
	paths := p.GetPaths()
	if len(paths) == 0 {
		return
	}
	ok = true
	min, max = paths[0].Bounds()
	paths = paths[1:]
	for _, path := range paths {
		path.ExtendBounds(&min, &max)
	}
	return
}
func (p *Agency) getOrAddRouteByTag(tag string) *Route {
	route, ok := p.Routes[tag]
	if !ok {
		route = NewRoute(tag)
		route.Agency = p
		p.Routes[tag] = route
	}
	return route
}
func (p *Agency) stopFromDefinitionElem(se *StopElement) (*Stop, error) {
	// We don't truly need the title, but it should be present,
	// and makes certain debugging easier.
	if len(se.Tag) == 0 || len(se.Title) == 0 || (se.Lat == 0 && se.Lon == 0) {
		return nil, fmt.Errorf(
			"stop element missing required data; elem: %#v", se)
	}
	stop, ok := p.Stops[se.Tag]
	if !ok {
		stop = NewStop(se.Tag)
		err := stop.initFromElement(se, p)
		if err != nil {
			return nil, err
		}
		p.Stops[se.Tag] = stop
	}
	err := stop.initFromElement(se, p)
	return stop, err
}
func (p *Agency) getOrAddLocation(latLon geo.Location) *Location {
	location, ok := p.Locations[latLon]
	if !ok {
		// Not providing a NewLocation function because this should be the only
		// function that creates locations.
		location = &Location{
			Location: latLon,
			Stops:    make(map[string]*Stop),
			Paths:    make(map[int]*Path),
			Agency:   p,
		}
		p.Locations[latLon] = location
	}
	return location
}

// Direction's are owned by a Route (doesn't make sense to share a direction
// object across routes), so after having been created by a Route, the Route
// adds it to the agency for convenient indexing.
func (p *Agency) addNewDirection(direction *Direction) {
	if _, ok := p.Directions[direction.Tag]; ok {
		panic(fmt.Errorf("Direction has already been added to the Agency: %#v", *direction))
	}
	p.Directions[direction.Tag] = direction
}

func hashFloat64(v float64, hasher hash.Hash64) {
	bits := math.Float64bits(v)
	var b [8]byte
	for i := range b {
		b[i] = byte(bits & 0xff)
		bits = bits >> 8
	}
	hasher.Write(b[:])
}

//func hashLocation(location *geo.Location, hasher hash.Hash64) {
//	hashFloat64(float64(location.Lat), hasher)
//	hashFloat64(float64(location.Lon), hasher)
//}
func hashLocations(locations []geo.Location) uint64 {
	hasher := fnv.New64a()
	for i := range locations {
		hashFloat64(float64(locations[i].Lat), hasher)
		hashFloat64(float64(locations[i].Lon), hasher)
	}
	return hasher.Sum64()
}
func equalLocationSlices(nbLocations []*Location,
	geoLocations []geo.Location) bool {
	if len(nbLocations) != len(geoLocations) {
		return false
	}
	for i := range nbLocations {
		if nbLocations[i].Location != geoLocations[i] {
			return false
		}
	}
	return true
}

// May reverse order of locations.
func (p *Agency) getOrAddPathByLocations(geoLocations []geo.Location) *Path {
	// NOTE: Used to reverse paths when necessary so that paths went from south
	// to north, then west to east, in the expectation that nextbus or mbta might
	// output paths in either order (i.e. a path might be used by either direction
	// of a route, or in either direction by various routes), but it appears that
	// paths always appear in the same order in the files (and may in fact always
	// be in the order that vehicles travel the route).
	//	// Put locations in a consistent order
	//	// (prefer south to north, then west to east).
	//	last := len(geoLocations) - 1
	//	if last > 0 &&
	//		(geoLocations[0].Lat > geoLocations[last].Lat ||
	//			(geoLocations[0].Lat == geoLocations[last].Lat &&
	//				geoLocations[0].Lon > geoLocations[last].Lon)) {
	//		// Reverse the order
	//		//		log.Printf("Reversing the order of points in a path")
	//		for i, j := 0, last; i < j; i, j = i+1, j-1 {
	//			geoLocations[i], geoLocations[j] = geoLocations[j], geoLocations[i]
	//		}
	//	}
	hash := hashLocations(geoLocations)
	paths, ok := p.PathsByHash[hash]
	if ok {
		for _, path := range paths {
			if equalLocationSlices(path.WayPoints, geoLocations) {
				if path.Hash != hash {
					panic(fmt.Errorf(
						"hashes should be equal; new hash: %v; existing path: %v; new locations %v",
						hash, path, geoLocations))
				}
				//				log.Printf("Found duplicate path")
				return path
			}
		}
	}
	p.lastPathIndex++
	if _, ok := p.PathsByIndex[p.lastPathIndex]; ok {
		log.Panicf("There is already a path with index %d", p.lastPathIndex)
	}
	path := NewPath()
	path.WayPoints = make([]*Location, len(geoLocations))
	path.Hash = hash
	path.Index = p.lastPathIndex
	for i := range geoLocations {
		path.WayPoints[i] = p.getOrAddLocation(geoLocations[i])
	}
	p.PathsByIndex[path.Index] = path
	p.PathsByHash[hash] = append(p.PathsByHash[hash], path)
	if len(p.PathsByHash[hash]) > 1 {
		log.Printf("Found %d paths with hash %v", len(p.PathsByHash[hash]), hash)
	}
	return path
}
