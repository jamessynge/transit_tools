package nextbus

import (
	"encoding/xml"
	"fmt"
	"github.com/jamessynge/transit_tools/geo"
	"github.com/golang/glog"
	//	"log"
)

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

  <route tag="76" title="76" scheduleClass="20130323" serviceClass="MoTuWeThFr" direction="Inbound">
    <header>
      <stop tag="85231">Lincoln Lab</stop>
      <stop tag="86179_1">Civil Air Terminal</stop>
    </header>
    <tr blockID="T76_45">
      <stop tag="85231" epochTime="21600000">06:00:00</stop>
*/

type HeaderElement struct {
	Stops []StopElement `xml:"stop"`
}
type TrElement struct {
	BlockID string        `xml:"blockID,attr"`
	Stops   []StopElement `xml:"stop"`
}
type PointElement struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
}
type PathElement struct {
	Points []*PointElement `xml:"point"`
}
type StopElement struct {
	Tag         string  `xml:"tag,attr"`
	Title       string  `xml:"title,attr"`
	ShortTitle  string  `xml:"shortTitle,attr"`
	Lat         float64 `xml:"lat,attr"`
	Lon         float64 `xml:"lon,attr"`
	StopId      string  `xml:"stopId,attr"`
	EpochTime   int64  `xml:"epochTime,attr"`
	ElementText string  `xml:",chardata"`
}
type DirectionElement struct {
	Tag      string        `xml:"tag,attr"`
	Title    string        `xml:"title,attr"`
	Name     string        `xml:"name,attr"`
	UseForUI bool          `xml:"useForUI,attr"`
	Stops    []StopElement `xml:"stop"`
}
type RouteElement struct {
	Tag           string              `xml:"tag,attr"`
	Title         string              `xml:"title,attr"`
	ShortTitle    string              `xml:"shortTitle,attr"`
	ScheduleClass string              `xml:"scheduleClass,attr"`
	ServiceClass  string              `xml:"serviceClass,attr"`
	Direction     string              `xml:"direction,attr"`
	Color         string              `xml:"color,attr"`
	OppColor      string              `xml:"oppositeColor,attr"`
	LatMin        float64             `xml:"latMin,attr"`
	LatMax        float64             `xml:"latMax,attr"`
	LonMin        float64             `xml:"lonMin,attr"`
	LonMax        float64             `xml:"lonMax,attr"`
	Stops         []*StopElement      `xml:"stop"`
	Directions    []*DirectionElement `xml:"direction"`
	Paths         []*PathElement      `xml:"path"`
	SchedHeader   *HeaderElement      `xml:"header"`
	Trips         []*TrElement        `xml:"tr"`
}
type VehicleElement struct {
	Id              string  `xml:"id,attr"`
	RouteTag        string  `xml:"routeTag,attr"`
	DirTag          string  `xml:"dirTag,attr"`
	Lat             float64 `xml:"lat,attr"`
	Lon             float64 `xml:"lon,attr"`
	SecsSinceReport int     `xml:"secsSinceReport,attr"`
	Predictable     bool    `xml:"predictable,attr"`
	Heading         int     `xml:"heading,attr"`
	SpeedKmHr       float64 `xml:"speedKmHr,attr"`
}
type LastTimeElement struct {
	Time int64 `xml:"time,attr"`
}
type ErrorElement struct {
	ShouldRetry bool   `xml:"shouldRetry,attr"`
	ElementText string `xml:",chardata"`
}
type BodyElement struct {
	Routes   []*RouteElement   `xml:"route"`
	Vehicles []*VehicleElement `xml:"vehicle"`
	LastTime *LastTimeElement  `xml:"lastTime"`
	Error    *ErrorElement
}

func UnmarshalNextbusXml(data []byte) (*BodyElement, error) {
	body := &BodyElement{}
	err := xml.Unmarshal([]byte(data), body)
	if err != nil {
		glog.Errorf("Unmarshal returned error: %s", err)
		glog.Errorf("Raw data: %q", data)
		glog.Errorf("Partially decoded: %#v", body)
		return nil, err
	}
	return body, nil
}

// Root element for command=vehicleLocations
type VehicleLocationsBodyElement struct {
	Error    *ErrorElement
	Vehicles []*VehicleElement `xml:"vehicle"`
	LastTime *LastTimeElement  `xml:"lastTime"`
}

func UnmarshalVehicleLocationsBytes(
	data []byte) (body *VehicleLocationsBodyElement, err error) {
	body = &VehicleLocationsBodyElement{}
	err = xml.Unmarshal([]byte(data), body)
	if err != nil {
		glog.Errorf("Unmarshal returned error: %s", err)
		glog.Errorf("Raw data: %q", data)
		glog.Errorf("Partially decoded: %#v", *body)
		body = nil
	}
	return
}

func maybeSetStringField(val string, field *string) (mismatch bool) {
	if len(val) != 0 {
		if len(*field) == 0 {
			*field = val
		} else if *field != val {
			return true
		}
	}
	return
}
func maybeSetLatitudeField(val float64, field *geo.Latitude) (mismatch bool) {
	if !(-90 <= val && val <= 90) {
		panic(fmt.Errorf("Latitude out of range: %v", val))
	}
	lat := geo.Latitude(val)
	if *field == 0 {
		*field = lat
	} else if *field != lat {
		mismatch = true
	}
	return
}
func maybeSetLongitudeField(val float64, field *geo.Longitude) (mismatch bool) {
	if !(-180 <= val && val <= 180) {
		panic(fmt.Errorf("Longitude out of range: %v", val))
	}
	lon := geo.Longitude(val)
	if *field == 0 {
		*field = lon
	} else if *field != lon {
		mismatch = true
	}
	return
}



