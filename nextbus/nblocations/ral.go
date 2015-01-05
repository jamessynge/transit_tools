package nblocations

import (

	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
)

type RecordAndLocation struct {
	// The record as returned by the CSV reader.
	Record []string
	// Our interpretation of the record as a vehicle location.
	nextbus.VehicleLocation
}

func (a *RecordAndLocation) SouthToNorthLess(b *RecordAndLocation) bool {
	return a.Location.Lat < b.Location.Lat
}
func SouthToNorthLess(a, b *RecordAndLocation) bool {
	return a.SouthToNorthLess(b)
}
func SortSouthToNorth(s []*RecordAndLocation) {
	less := func(i, j int) bool {
		return SouthToNorthLess(s[i], s[j])
	}
	swap := func(i, j int) {
		s[i], s[j] = s[j], s[i]
	}
	util.Sort3(len(s), less, swap)
}

func (a *RecordAndLocation) WestToEastLess(b *RecordAndLocation) bool {
	return a.Location.Lon < b.Location.Lon
}
func WestToEastLess(a, b *RecordAndLocation) bool {
	return a.Location.Lon < b.Location.Lon
}
func SortWestToEast(s []*RecordAndLocation) {
	less := func(i, j int) bool {
		return WestToEastLess(s[i], s[j])
	}
	swap := func(i, j int) {
		s[i], s[j] = s[j], s[i]
	}
	util.Sort3(len(s), less, swap)
}
