package nextbus

import (
	"fmt"
)

type Direction struct {
	Tag        string
	Title      string
	Name       string
	UseForUI   bool
	Route      *Route
	Stops      []*Stop
	StopsIndex map[string]int // value is index of stop in Direction.Stops
}

func NewDirection(tag string) *Direction {
	return &Direction{
		Tag:        tag,
		StopsIndex: make(map[string]int),
	}
}

func (p *Direction) initFromElement(elem *DirectionElement) error {
	mismatch := maybeSetStringField(elem.Tag, &p.Tag)
	mismatch = maybeSetStringField(elem.Title, &p.Title) || mismatch
	mismatch = maybeSetStringField(elem.Name, &p.Name) || mismatch

	// Don't (currently) have a way to tell if these UseForUI fields are set,
	// or are at their default values.
	if p.UseForUI && !elem.UseForUI {
		mismatch = true
	} else {
		p.UseForUI = elem.UseForUI
	}

	if mismatch {
		return fmt.Errorf(
			"mismatch between directionElement and Direction:\n\t%#v\t%#v",
			*elem, *p)
	}
	return nil
}
