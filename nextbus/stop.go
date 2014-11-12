package nextbus

import (
	"github.com/jamessynge/transit_tools/compare"
	"fmt"
	"github.com/jamessynge/transit_tools/geo"
	"strings"
)

type Stop struct {
	Tag        string
	Id         string
	Title      string
	ShortTitle string
	Location   *Location
	Routes     map[string]*Route
	Directions map[string]*Direction
}

func NewStop(tag string) *Stop {
	return &Stop{
		Tag:        tag,
		Directions: make(map[string]*Direction),
		Routes:     make(map[string]*Route),
	}
}

func (p *Stop) GoString() string {
	var strs []string
	if len(p.Tag) > 0 {
		strs = append(strs, fmt.Sprintf("Tag: %#v", p.Tag))
	}
	if len(p.Id) > 0 {
		strs = append(strs, fmt.Sprintf("Id: %#v", p.Id))
	}
	if len(p.Title) > 0 {
		strs = append(strs, fmt.Sprintf("Title: %#v", p.Title))
	}
	if len(p.ShortTitle) > 0 {
		strs = append(strs, fmt.Sprintf("ShortTitle: %#v", p.ShortTitle))
	}
	if p.Location != nil {
		strs = append(strs,
			fmt.Sprintf("Location: %#v", p.Location.Location))
	}
	return fmt.Sprint("{", strings.Join(strs, ", "), "}")
}

func (p *Stop) initFromElement(se *StopElement, agency *Agency) error {
	var mismatch bool
	var field string
	if maybeSetStringField(se.Tag, &p.Tag) {
		mismatch = true
		field = "Tag"
	} else if maybeSetStringField(se.StopId, &p.Id) {
		mismatch = true
		field = "Id"
	} else if maybeSetStringField(se.Title, &p.Title) {
		mismatch = true
		field = "Title"
	} else if maybeSetStringField(se.ShortTitle, &p.ShortTitle) {
		mismatch = true
		field = "ShortTitle"
	}

	if !mismatch && (se.Lat != 0 || se.Lon != 0) {
		latLon, err := geo.LocationFromFloat64s(se.Lat, se.Lon)
		if err != nil {
			return fmt.Errorf("error while initializing from element: "+
				"%v\nstop element: %v", err, *se)
		}
		if p.Location == nil {
			p.Location = agency.getOrAddLocation(latLon)
			if len(p.Tag) > 0 {
				p.Location.Stops[p.Tag] = p
			}
		} else {
			stopLatLon := p.Location.Location
			if !compare.NearlyEqual(float64(stopLatLon.Lat), float64(latLon.Lat)) {
				mismatch = true
				field = "Latitude"
			} else if !compare.NearlyEqual(float64(stopLatLon.Lon), float64(latLon.Lon)) {
				mismatch = true
				field = "Longitude"
			}
		}
	}
	if mismatch {
		return fmt.Errorf(
			"mismatch in field %s between stopElement and Stop:\n\t%#v\n\t%#v",
			field, *se, p)
	}
	return nil
}
func (p *Stop) IsDefined() bool {
	return len(p.Tag) > 0 && len(p.Title) > 0
}
