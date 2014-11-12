package nextbus

import (
	"github.com/jamessynge/transit_tools/compare"
	"github.com/jamessynge/transit_tools/geo"
	"testing"
	"time"
)

/*
import "encoding/xml"

func TestParseXmlVehicleElement(t *testing.T) {
	message_time := time.Unix(1350562779, 0)
	vl, err := ParseXmlVehicleElement(nil, message_time)

	start_element := &xml.StartElement{
		xml.Name{"", "vehicle"},
		[]xml.Attr{
			{xml.Name{"", "id"}, "0199"},
			{xml.Name{"", "routeTag"}, "64"},
			{xml.Name{"", "dirTag"}, "64_1_var0"},
			{xml.Name{"", "lat"}, "42.3685977"},
			{xml.Name{"", "lon"}, "-71.0991791"},
			{xml.Name{"", "secsSinceReport"}, "20"},
			{xml.Name{"", "predictable"}, "true"},
			{xml.Name{"", "heading"}, "118"},
			{xml.Name{"", "speedKmHr"}, "0.0"},
		},
	}
	vl, err = ParseXmlVehicleElement(start_element, message_time)
	if err != nil {
		t.Fatal(err)
	}
	if vl == nil {
		t.Fatal("VehicleLocation not returned")
	}
	t.Logf("      vl: %v", *vl)
	expected := VehicleLocation{
		VehicleId: "0199",
		RouteTag:  "64",
		DirTag:    "64_1_var0",
		Location: geo.Location{
			Lat: geo.Latitude(42.3685977),
			Lon: geo.Longitude(-71.0991791),
		},
		Time:    time.Unix(1350562759, 0),
		Heading: 118,
	}
	t.Logf("expected: %v", expected)

	compare.ExpectEqual(t.Error, expected, *vl)
}*/

func TestParseXmlVehicleLocations(t *testing.T) {
	s := ""
	report, err := ParseXmlVehicleLocations([]byte(s))
	if err == nil {
		t.Error("Expected EOF error")
	}
	if report != nil {
		t.Error("report not expected: ", report)
	}

	s = `<?xml version="1.0" encoding="utf-8" ?>
<body copyright="All data copyright MBTA 2012.">
<vehicle id="0199" routeTag="64" dirTag="64_1_var0" lat="42.3685977" lon="-71.0991791" secsSinceReport="20" predictable="true" heading="118" speedKmHr="0.0"/>
<vehicle id="0877" routeTag="451" dirTag="451_1_var0" lat="42.5513283" lon="-70.878608" secsSinceReport="35" predictable="true" heading="160" speedKmHr="0.0"/>
<lastTime time="1350562779906"/>
</body>`
	report, err = ParseXmlVehicleLocations([]byte(s))
	if err != nil {
		t.Errorf("error returned, not locations: %v", err)
	}
	if report == nil {
		t.Error("report not returned")
	}
	if len(report.VehicleLocations) != 2 {
		t.Errorf("2 Locations not expected, not %d", len(report.VehicleLocations))
	} else {
		expected := []*VehicleLocation{
			&VehicleLocation{
				VehicleId: "0199",
				RouteTag:  "64",
				DirTag:    "64_1_var0",
				Location: geo.Location{
					Lat: geo.Latitude(42.3685977),
					Lon: geo.Longitude(-71.0991791),
				},
				Time:    time.Unix(1350562779, 906000000),
				Heading: 118,
			},
			&VehicleLocation{
				VehicleId: "0877",
				RouteTag:  "451",
				DirTag:    "451_1_var0",
				Location: geo.Location{
					Lat: geo.Latitude(42.5513283),
					Lon: geo.Longitude(-70.878608),
				},
				Time:    time.Unix(1350562764, 906000000),
				Heading: 160,
			},
		}
		t.Logf("*expected[0]:\n%v", *expected[0])
		t.Logf("*expected[1]:\n%v", *expected[1])
		compare.ExpectEqual(t.Error, expected, report.VehicleLocations)
	}
}
