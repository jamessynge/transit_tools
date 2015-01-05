package nextbus

import (
	"fmt"
	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/util"
	"sort"
	"strconv"
	"time"
)

type VehicleLocation struct {
	VehicleId string
	RouteTag  string
	DirTag    string
	Time      time.Time
	geo.Location
	Route     *Route
	Direction *Direction
	Heading   geo.Heading
	// Consider adding SpeedKmHr in support of those agencies which report it.
}

type VehicleLocationsReport struct {
	LastTime         time.Time
	ErrorMessage     string
	ShouldRetry      bool
	VehicleLocations []*VehicleLocation
}

func (u *VehicleLocation) IsSameReportExceptTime(v *VehicleLocation) bool {
	if v == nil || !u.Location.SameLocation(v.Location) ||
		u.Heading != v.Heading || u.VehicleId != v.VehicleId ||
		u.RouteTag != v.RouteTag || u.DirTag != v.DirTag {
		return false
	}
	return true
}

func (u *VehicleLocation) IsAlmostSameTime(v *VehicleLocation) bool {
	diff := u.Time.Sub(v.Time).Seconds()
	return -2.0 <= diff && diff <= 2.0
}

func (u *VehicleLocation) IsSameVehiclePosition(v *VehicleLocation) bool {
	if v == nil || !u.Location.SameLocation(v.Location) ||
		u.Heading != v.Heading || u.VehicleId != v.VehicleId {
		return false
	}
	return true
}

func (u *VehicleLocation) IsSameReport(v *VehicleLocation) bool {
	return u.IsSameReportExceptTime(v) && u.IsAlmostSameTime(v)
}

func (u *VehicleLocation) UnixMilliseconds() int64 {
	s, n := u.Time.Unix(), u.Time.Nanosecond()
	return s*1000 + int64(n)/1000000
}

func VehicleCSVFieldNames() (fields []string) {
	fields = append(fields, "unix_ms")
	fields = append(fields, "date time")
	fields = append(fields, "vehicle id")
	fields = append(fields, "route tag")
	fields = append(fields, "direction tag")
	fields = append(fields, "heading")
	fields = append(fields, "latitude")
	fields = append(fields, "longitude")
	return
}

//   timestamp, date time, vehicle id, route tag, direction tag, heading, latitude, longitude
func (u *VehicleLocation) ToCSVFields() (fields []string) {
	fields = append(fields, fmt.Sprintf("%d", u.UnixMilliseconds()))
	fields = append(fields, u.Time.Format("20060102 150405"))
	fields = append(fields, u.VehicleId)
	fields = append(fields, u.RouteTag)
	fields = append(fields, u.DirTag)
	fields = append(fields, fmt.Sprint(u.Heading))
	fields = append(fields, fmt.Sprint(u.Lat))
	fields = append(fields, fmt.Sprint(u.Lon))
	return
}

const (
	Jan_1_2000_UTC = 946684800000
	Jan_1_2100_UTC = 4102444800000
)

var (
	PROGRAM_START_TIME               = time.Now()
	ONE_YEAR                         = time.Duration(time.Hour * 24 * 365)
	ONE_YEAR_PLUS_PROGRAM_START_TIME = PROGRAM_START_TIME.Add(ONE_YEAR)
	MILLIS_LIMIT                     = uint64(ONE_YEAR_PLUS_PROGRAM_START_TIME.Unix() * 1000)
)

func CSVFieldsIntoVehicleLocation(
	fields []string, loc *VehicleLocation) error {
	if len(fields) != 8 {
		return fmt.Errorf("Expected 8 fields, not %d", len(fields))
	}

	millis, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return err
	} else if millis < Jan_1_2000_UTC {
		return fmt.Errorf("Timestamp too low: %v", millis)
	} else if MILLIS_LIMIT < millis {
		return fmt.Errorf("Timestamp too high: %v", millis)
	}
	loc.Time = time.Unix(int64(millis/1000), int64((millis%1000)*1000000))
	loc.VehicleId = fields[2]
	loc.RouteTag = fields[3]
	loc.DirTag = fields[4]

	// Ignoring error from parsing heading (very
	// occasionally have negative values).
	loc.Heading, _ = geo.ParseHeading(fields[5])

	lat, err := geo.ParseLatitude(fields[6])
	if err != nil {
		return err
	}
	lon, err := geo.ParseLongitude(fields[7])
	if err != nil {
		return err
	}
	loc.Location = geo.Location{Lat: lat, Lon: lon}

	return nil
}

func CSVFieldsToVehicleLocation(fields []string) (*VehicleLocation, error) {
	loc := new(VehicleLocation)
	err := CSVFieldsIntoVehicleLocation(fields, loc)
	return loc, err
}

func ConvertVehicleElementToVehicleLocation(
	elem *VehicleElement, reportTimeMs int64) (*VehicleLocation, error) {
	loc, err := geo.LocationFromFloat64s(elem.Lat, elem.Lon)
	if err != nil {
		return nil, err
	}
	heading, err := geo.HeadingFromInt(elem.Heading)
	//	if err != nil {
	//		return nil, err
	//	}

	result := &VehicleLocation{}
	result.VehicleId = elem.Id
	result.RouteTag = elem.RouteTag
	result.DirTag = elem.DirTag
	result.Time = util.UnixMillisToTime(reportTimeMs - int64(elem.SecsSinceReport*1000))
	result.Location = loc
	result.Heading = heading
	return result, err
}

/*
func newCapacity1(capacity int) int {
	if capacity < 4 {
		return 4
	} else {
		capacity += 1
		capacity = capacity | capacity>>1
		capacity = capacity | capacity>>2
		capacity = capacity | capacity>>4
		capacity = capacity | capacity>>8
		capacity = capacity | capacity>>16
		return capacity + 1
	}
}

func appendStartElement(slice []*xml.StartElement, elem *xml.StartElement) []*xml.StartElement {
	m := len(slice)
	n := m + 1
	// if necessary, reallocate
	if n > cap(slice) {
		// allocate more than is needed, for future growth.
		new_cap := newCapacity1(cap(slice))
		new_slice := make([]*xml.StartElement, new_cap)
		copy(new_slice, slice)
		slice = new_slice
	}
	slice = slice[0:n]
	slice[m] = elem
	return slice
}

func getAttr(elem xml.StartElement, attr_name string) *string {
	for _, attr := range elem.Attr {
		if attr.Name.Local == attr_name {
			return &attr.Value
		}
	}
	return nil
}
*/
func ConvertVehicleLocationsBodyToReport(
	body *VehicleLocationsBodyElement) (*VehicleLocationsReport, error) {
	response := &VehicleLocationsReport{}
	if body.Error != nil {
		response.ErrorMessage = body.Error.ElementText
		response.ShouldRetry = body.Error.ShouldRetry
	}
	var lastTimeMs int64
	var errors []error
	if body.LastTime != nil {
		lastTimeMs = body.LastTime.Time
		if lastTimeMs <= 0 {
			errors = append(errors, fmt.Errorf("Invalid lastTime: %v", lastTimeMs))
		} else {
			response.LastTime = util.UnixMillisToTime(lastTimeMs)
		}
	}
	if len(body.Vehicles) > 0 {
		youngestAge := body.Vehicles[0].SecsSinceReport
		//		log.Printf("Initial youngestAge: %d", youngestAge)
		for _, elem := range body.Vehicles[1:] {
			if youngestAge > elem.SecsSinceReport {
				youngestAge = elem.SecsSinceReport
				//				log.Printf("New youngestAge: %d", youngestAge)
			} else {
				//				log.Printf("Not new youngestAge: %d", elem.SecsSinceReport)
			}
		}
		// response.LastTime is the time at which the most recent vehicle location
		// report was received (i.e. the one that has the youngest age). Add the
		// youngestAge to determine the time when the report was generated;
		// we can then compute the time at which each VehicleLocation
		// report was received.
		reportTimeMs := lastTimeMs + int64(youngestAge*1000)
		//		log.Printf("  LastTime:   %s", response.LastTime)
		//		log.Printf("  lastTimeMs: %d", lastTimeMs)
		//		log.Printf("reportTimeMs: %d", reportTimeMs)

		response.VehicleLocations = make([]*VehicleLocation, len(body.Vehicles))
		for i, elem := range body.Vehicles {
			vl, err := ConvertVehicleElementToVehicleLocation(elem, reportTimeMs)
			if err != nil {
				errors = append(errors, err)
			}
			response.VehicleLocations[i] = vl
		}
	}
	return response, nil
}

/*
func ReadXmlVehicleLocations(r io.Reader) (*VehicleLocationsReport, error) {
	decoder := xml.NewDecoder(r)
	vehicle_elements := make([]*xml.StartElement, 0, 2)
	response := &VehicleLocationsReport{}
	haveTime := false
	youngestAge := -1
	for {
		t, err := decoder.RawToken()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		fmt.Printf("RawToken of type %T -> %v\n", t, t)
		switch e := t.(type) {
		case xml.StartElement:
			if e.Name.Local == "vehicle" {
				vehicle_elements = appendStartElement(vehicle_elements, &e)
				attr := getAttr(e, "secsSinceReport")
				if attr != nil {
					secs, err := strconv.ParseUint(*attr, 10, 16)
					if err == nil {
						if youngestAge == -1 || youngestAge > int(secs) {
							youngestAge = int(secs)
						}
					}
				}
			} else if e.Name.Local == "lastTime" {
				av := getAttr(e, "time")
				if av == nil {
					return nil, fmt.Errorf("time attribute missing: %v", e)
				}
				lastTimeMs, err := strconv.ParseInt(*av, 10, 64)
				if err != nil || lastTimeMs <= 0 {
					return nil, fmt.Errorf("invalid time attribute: %v\nerr: %v", e, err)
				}
				lastTimeSec := lastTimeMs / 1000
				lastTimeNs := (lastTimeMs % 1000) * 1000000
				response.LastTime = time.Unix(lastTimeSec, lastTimeNs)
				haveTime = true
			} else if e.Name.Local == "Error" {
				// Consume the next xml.CharData token.
				nt, err := decoder.RawToken()
//				switch e := nt.(type) {
//				case xml.StartElement:



				if err == nil {
					fmt.Printf("After Error, RawToken of type %T -> %v\n", nt, nt)
				}
			}
		}
	}
	if !haveTime {
		return nil, fmt.Errorf("lastTime element missing")
	}
	if youngestAge > 0 {
		// We don't have the server time at which the message was generated, but
		// we know how long before the message was generated that each location
		// report arrived ("secsSinceReport").  lastTime is the unix time, milliseconds,
		// that the most recent report arrived, so we can add the smallest such age to
		// lastTime to come up with, roughly, the server time at which the message
		// was generated.
		delta := time.Duration(youngestAge) * time.Second
		response.LastTime = response.LastTime.Add(delta)
	}
	response.Locations = make([]*VehicleLocation, len(vehicle_elements))
	for i, elem := range vehicle_elements {
		vl, err := ParseXmlVehicleElement(elem, response.LastTime)
		if err != nil {
			return nil, err
		}
		response.Locations[i] = vl
	}
	return response, nil
}
*/

func ParseXmlVehicleLocations(data []byte) (*VehicleLocationsReport, error) {
	bodyElem, err := UnmarshalVehicleLocationsBytes(data)
	if err != nil {
		return nil, err
	}
	return ConvertVehicleLocationsBodyToReport(bodyElem)
}

type vehicleLocationPtrsSlice struct {
	less func(a, b *VehicleLocation) bool
	data []*VehicleLocation
}

func (s vehicleLocationPtrsSlice) Len() int {
	return len(s.data)
}

func (s vehicleLocationPtrsSlice) Less(i, j int) bool {
	a, b := s.data[i], s.data[j]
	return s.less(a, b)
}

func (s vehicleLocationPtrsSlice) Swap(i, j int) {
	s.data[j], s.data[i] = s.data[i], s.data[j]
}

func SortVehicleLocations(s []*VehicleLocation, less func(a, b *VehicleLocation) bool) {
	vlps := vehicleLocationPtrsSlice{less, s}
	sort.Sort(vlps)
}

func SortVehicleLocationsByDate(s []*VehicleLocation) {
	less := func(a, b *VehicleLocation) bool {
		an, bn := a.Time.UnixNano(), b.Time.UnixNano()
		return an < bn
	}
	SortVehicleLocations(s, less)
}

func SortVehicleLocationsById(s []*VehicleLocation) {
	less := func(a, b *VehicleLocation) bool {
		return a.VehicleId < b.VehicleId
	}
	SortVehicleLocations(s, less)
}

func SortVehicleLocationsByDateAndId(s []*VehicleLocation) {
	less := func(a, b *VehicleLocation) bool {
		an, bn := a.Time.UnixNano(), b.Time.UnixNano()
		if an < bn {
			return true
		} else if an == bn && a.VehicleId < b.VehicleId {
			return true
		}
		return false
	}
	SortVehicleLocations(s, less)
}

func SortVehicleLocationsByIdAndDate(s []*VehicleLocation) {
	less := func(a, b *VehicleLocation) bool {
		if a.VehicleId < b.VehicleId {
			return true
		} else if a.VehicleId > b.VehicleId {
			return false
		}
		an, bn := a.Time.UnixNano(), b.Time.UnixNano()
		return an < bn
	}
	SortVehicleLocations(s, less)
}
