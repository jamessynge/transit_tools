// Support for unmarshalling the message produced by
//   http://webservices.nextbus.com/service/publicXMLFeed?command=vehicleLocations&a=mbta

package unmarshal_vehicle_locations

import (
	"encoding/xml"
	"io"
)

type VehicleElement struct {
	Id              string  `xml:"id,attr"`
	RouteTag        string  `xml:"routeTag,attr"`
	DirTag          string  `xml:"dirTag,attr"`
	Lat             float64 `xml:"lat,attr"`
	Lon             float64 `xml:"lon,attr"`
	SecsSinceReport float64 `xml:"secsSinceReport,attr"`
	Predictable     bool    `xml:"predictable,attr"`
	Heading         int     `xml:"heading,attr"`
	SpeedKmHr       float64 `xml:"speedKmHr,attr"`
}
type LastTimeElement struct {
	// Unix epoch time (milliseconds) at which most recent report arrived (GMT).
	Time int64 `xml:"time,attr"`
}
type ErrorElement struct {
	ShouldRetry bool   `xml:"shouldRetry,attr"`
	ElementText string `xml:"chardata"`
}
type BodyElement struct {
	Error    *ErrorElement
	Vehicles []*VehicleElement `xml:"vehicle"`
	LastTime *LastTimeElement  `xml:"lastTime"`
}

func UnmarshalVehicleLocationsBytes(data []byte) (*BodyElement, error) {
	body := &BodyElement{}
	err := xml.Unmarshal([]byte(data), body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
func UnmarshalVehicleLocationsReader(r io.Reader) (*BodyElement, error) {
	decoder := xml.NewDecoder(r)
	body := &BodyElement{}
	err := decoder.Decode(body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
