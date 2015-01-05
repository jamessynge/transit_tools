package nextbus

import (
	"encoding/xml"
	"fmt"
	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/util"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

type Route struct {
	Tag        string
	Title      string
	ShortTitle string
	Color      string
	OppColor   string
	LatMin     geo.Latitude
	LatMax     geo.Latitude
	LonMin     geo.Longitude
	LonMax     geo.Longitude
	Stops      map[string]*Stop
	Directions map[string]*Direction
	Paths      []*Path
	Agency     *Agency
}

func NewRoute(tag string) *Route {
	return &Route{
		Tag:        tag,
		Directions: make(map[string]*Direction),
		Stops:      make(map[string]*Stop),
	}
}

// Direction's are owned by a Route (doesn't make sense
// to share a direction object across routes).
func (p *Route) getOrAddDirectionByTag(tag string) *Direction {
	direction, ok := p.Directions[tag]
	if !ok {
		//		log.Printf("Adding direction %s to route '%s' (tag %s)", tag, p.Title, p.Tag)
		direction = NewDirection(tag)
		direction.Route = p
		p.Directions[tag] = direction
		p.Agency.addNewDirection(direction)
	}
	return direction
}

//func (p *Route) getOrAddStopByTag(tag string) *Stop {
//	stop, ok := p.Stops[tag]
//	if !ok {
//		// Stops are shared across the Routes of an Agency.
//		stop = p.Agency.getOrAddStopByTag(tag)
//		p.Stops[tag] = stop
//		stop.Routes[p.Tag] = p
//	}
//	return stop
//}

func (p *Route) parseRouteConfigStopDefs(re *RouteElement, errors *[]error) {
	//	log.Printf("Route.parseRouteConfigStopDefs for route '%s' (tag %s)", p.Title, p.Tag)
	for _, se := range re.Stops {
		//		log.Printf(" Processing definition of stop '%s' (tag %s)", se.Title, se.Tag)
		stop, err := p.Agency.stopFromDefinitionElem(se)
		if err != nil {
			*errors = append(*errors, err)
		}
		if stop != nil {
			p.Stops[se.Tag] = stop
		}
	}
	//	log.Printf("Route.parseRouteConfigStopDefs DONE for route '%s' (tag %s)", p.Title, p.Tag)
}

func (p *Route) parseRouteConfigDirectionDefs(re *RouteElement, errors *[]error) {
	//	log.Printf("Route.parseRouteConfigDirectionDefs for route '%s' (tag %s)", p.Title, p.Tag)
	for _, elem := range re.Directions {
		//		log.Printf(" Processing definition of direction '%s' (tag %s)", elem.Title, elem.Tag)
		if len(elem.Tag) == 0 || len(elem.Title) == 0 {
			*errors = append(*errors, fmt.Errorf(
				"direction element missing expected data; elem: %#v", elem))
			continue
		}
		direction := p.getOrAddDirectionByTag(elem.Tag)
		err := direction.initFromElement(elem)
		if err != nil {
			*errors = append(*errors, err)
		}
		firstError := true
		for ndx, stopElem := range re.Stops {
			//			log.Printf("  Adding stop with tag %s", stopElem.Tag)
			stop, ok := p.Stops[stopElem.Tag]
			if ok {
				// Good, stop already defined.
				_, ok = direction.StopsIndex[stop.Tag]
				if !ok {
					// Good, stop not already added to direction.
					direction.StopsIndex[stop.Tag] = len(direction.Stops)
					direction.Stops = append(direction.Stops, stop)
					continue
				}
				err = fmt.Errorf("duplicate stop[%d]: %#v", ndx, stopElem)
			} else {
				err = fmt.Errorf("unknown stop[%d]: %#v", ndx, stopElem)
			}
			if firstError {
				*errors = append(*errors, fmt.Errorf(
					"error(s) in stop list for direction '%s'", direction.Title))
				firstError = false
			}
			*errors = append(*errors, err)
		}
	}
	//	log.Printf("Route.parseRouteConfigDirectionDefs DONE for route '%s' (tag %s)", p.Title, p.Tag)
}

func (p *Route) parseRouteConfigPathDefs(re *RouteElement, errors *[]error) {
	//	log.Printf("Route.parseRouteConfigPathDefs for route '%s' (tag %s)", p.Title, p.Tag)
	for _, elem := range re.Paths {
		//		log.Printf(" Processing path[%d] of %d points", ndx, len(elem.Points))
		if len(elem.Points) == 0 {
			*errors = append(*errors, fmt.Errorf(
				"direction element missing expected data; elem: %#v", elem))
			continue
		}
		// TODO Validate locations are inside declared route bounds.
		geoLocations := ToGeoLocations(elem.Points, errors)
		path := p.Agency.getOrAddPathByLocations(geoLocations)
		path.Routes[p.Tag] = p
		p.Paths = append(p.Paths, path)
	}
	//	log.Printf("Route.parseRouteConfigPathDefs DONE for route '%s' (tag %s)", p.Title, p.Tag)
}

func (p *Route) BoundsInitialized() bool {
	return p.LatMin < p.LatMax && p.LonMin < p.LonMax
}

//
//
//type pointElement struct {
//	Lat float64 `xml:"lat,attr"`
//	Lon float64 `xml:"lon,attr"`
//}
//type pathElement struct {
//	Points []*pointElement `xml:"point"`
//}

//parseRouteConfigPathDefs

func (p *Route) initFromElement(elem *RouteElement) error {
	mismatch := maybeSetStringField(elem.Tag, &p.Tag)
	mismatch = maybeSetStringField(elem.Title, &p.Title) || mismatch
	mismatch = maybeSetStringField(elem.ShortTitle, &p.ShortTitle) || mismatch
	mismatch = maybeSetStringField(elem.Color, &p.Color) || mismatch
	mismatch = maybeSetStringField(elem.OppColor, &p.OppColor) || mismatch

	if elem.LatMin != 0 || elem.LatMax != 0 || elem.LonMin != 0 || elem.LonMax != 0 {
		mismatch = maybeSetLatitudeField(elem.LatMin, &p.LatMin) || mismatch
		mismatch = maybeSetLatitudeField(elem.LatMax, &p.LatMax) || mismatch
		mismatch = maybeSetLongitudeField(elem.LonMin, &p.LonMin) || mismatch
		mismatch = maybeSetLongitudeField(elem.LonMax, &p.LonMax) || mismatch
	}
	if mismatch {
		return fmt.Errorf("mismatch between routeElement and Route: %v\t%v", *elem, *p)
	}

	//	log.Printf("Route.initFromElement exiting: %v", *p)
	return nil
}

/*
<?xml version="1.0" encoding="utf-8" ?>
<body copyright="All data copyright MBTA 2013.">
<route tag="76" title="76" color="9933cc" oppositeColor="ffffff"
  latMin="42.3954299" latMax="42.4628099" lonMin="-71.29118" lonMax="-71.14248">
<stop tag="141" title="Alewife Station Busway"
      lat="42.3954299" lon="-71.14248" stopId="00141"/>
<direction tag="76_0_var0" title="Hanscom Civil Airport via Lexington Center" name="Outbound" useForUI="true">
  <stop tag="141" />
  <stop tag="2480" />
</direction>
<path>
<point lat="42.39912" lon="-71.1461999"/>
<point lat="42.4003" lon="-71.15035"/>
</path>
<path>
<point lat="42.40541" lon="-71.16511"/>
<point lat="42.40529" lon="-71.1645"/>
</path>
</route>
</body>
*/
func ParseRouteConfigXml(agency *Agency, data []byte) (route *Route, err error) {
	if agency == nil {
		return nil, fmt.Errorf("agency must be specified")
	}
	var body BodyElement
	err = xml.Unmarshal([]byte(data), &body)
	if err != nil {
		fmt.Printf("error: %v", err)
		return nil, err
	}
	//	log.Printf("route body: %#v", body)
	if len(body.Routes) == 0 {
		return nil, fmt.Errorf("no route element inside body element")
	}
	var errors []error
	if len(body.Routes) > 1 {
		errors = append(errors, fmt.Errorf(
			"expected exactly one route in route configuration xml, not %d",
			len(body.Routes)))
	}
	re := body.Routes[0]
	if len(re.Tag) == 0 || len(re.Title) == 0 || len(re.Directions) == 0 ||
		(re.LatMin == 0 && re.LatMax == 0 && re.LonMin == 0 && re.LonMax == 0) {
		errors = append(errors, fmt.Errorf(
			"route element missing expected data; elem: %#v", re))
		return nil, fmt.Errorf(util.JoinErrors(errors, "\n"))
	}

	route = agency.getOrAddRouteByTag(re.Tag)
	//	log.Printf("Got route from agency: %v", *route)

	err = route.initFromElement(re)
	if err != nil {
		errors = append(errors, err)
	}

	if len(route.Stops) == 0 && len(route.Directions) == 0 &&
		len(route.Paths) == 0 {
		route.parseRouteConfigStopDefs(re, &errors)
		route.parseRouteConfigDirectionDefs(re, &errors)
		route.parseRouteConfigPathDefs(re, &errors)
	} else {
		log.Printf("Route '%s' configuration already loaded", re.Tag)
	}
	if len(errors) == 0 {
		return route, nil
	}
	log.Printf("Encountered %d errors while parsing route '%s' (tag %s)", len(errors), re.Title, re.Tag)
	return route, fmt.Errorf("%s", util.JoinErrors(errors, "\n"))
}

func ParseRouteConfigFile(agency *Agency, filePath string) (route *Route, err error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	route, err = ParseRouteConfigXml(agency, data)
	if err == nil {
		log.Printf("Done parsing %s    (tag=%s, title='%s')", filePath, route.Tag, route.Title)
		return
	}
	return
}

func ParseRouteConfigsDir(agency *Agency, dirPath string) (err error) {
	files, err := ioutil.ReadDir(dirPath)
	for _, elem := range files {
		//		log.Printf("Next file: %s", elem.Name())
		if !strings.HasSuffix(elem.Name(), ".xml") {
			continue
		}
		filePath := filepath.Join(dirPath, elem.Name())
		//		log.Printf("Parsing %s", filePath)
		_, err = ParseRouteConfigFile(agency, filePath)
		if err != nil {
			log.Printf("Error(s) found while parsing %s", filePath)
			return
		}
		//		log.Printf("Done parsing %s", filePath)
	}
	return
}
