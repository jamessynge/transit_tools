package nextbus

import (
    "testing"
    "github.com/jamessynge/transit_tools/compare"
)

func TestUnmarshalNextbusXml_Empty(t *testing.T) {
	s := ""
	body, err := UnmarshalNextbusXml([]byte(s))
	if err == nil {
		t.Error("Expected EOF error")
	}
	if body != nil {
		t.Error("body not expected: ", body)
	}
}

// http://webservices.nextbus.com/service/publicXMLFeed?command=routeList&a=mbta
func TestUnmarshalNextbusXml_RouteList(t *testing.T) {
	expected := &BodyElement{
		Routes: []*RouteElement{
			&RouteElement{Tag: "1", Title: "1"},
			&RouteElement{Tag: "442", Title: "442"},
			&RouteElement{Tag: "441442", Title: "441/442"},
			&RouteElement{Tag: "701", Title: "Ct1"},
			&RouteElement{Tag: "746", Title: "Silver Line Waterfront"},
			&RouteElement{Tag: "741", Title: "Silver Line SL1"},
			&RouteElement{Tag: "9701", Title: "9701"},
		},
	}
	s := `<body copyright="All data copyright MBTA 2014.">
<route tag="1" title="1"/><route tag="442" title="442"/>
<route tag="441442" title="441/442"/><route tag="701" title="Ct1"/>
<route tag="746" title="Silver Line Waterfront"/>
<route tag="741" title="Silver Line SL1"/>
<route tag="9701" title="9701"/></body>`
	body, err := UnmarshalNextbusXml([]byte(s))
	if err != nil {
		t.Error("Unxpected error:", err)
	}
	if body == nil {
		t.Error("nil body not expected")
		return
	}
	compare.ExpectEqual(t.Error, expected, body)
}

// http://webservices.nextbus.com/service/publicXMLFeed?command=routeConfig&a=mbta&r=76
func TestUnmarshalNextbusXml_RouteConfig(t *testing.T) {
	expected := &BodyElement{
		Routes: []*RouteElement{
			&RouteElement{
				Tag: "76",
				Title: "76",
				Color: "9933cc",
				OppColor: "ffffff",
				LatMin: 42.3954299,
				LatMax: 42.4628099,
				LonMin: -71.29118,
				LonMax: -71.14248,
				Stops: []*StopElement{
					&StopElement{
						Tag: "141",
						Title: "Alewife Station Busway",
						Lat: 42.3954299,
						Lon: -71.14248,
						StopId: "00141",
					},
				},
				Directions: []*DirectionElement{
					&DirectionElement{
						Tag: "76_0_var0",
						Title: "Hanscom Civil Airport via Lexington Center",
						Name: "Outbound",
						UseForUI: true,
						Stops: []StopElement{
							StopElement{Tag: "141"},
							StopElement{Tag: "2480"},
						},
					},
				},
				Paths: []*PathElement{
					&PathElement{
						Points: []*PointElement{
							&PointElement{Lat: 42.39912, Lon: -71.1461999},
							&PointElement{Lat: 42.4003, Lon: -71.15035},
						},
					},
					&PathElement{
						Points: []*PointElement{
							&PointElement{Lat: 42.40541, Lon: -71.16511},
							&PointElement{Lat: 42.40529, Lon: -71.1645},
						},
					},
				},
			},
		},
	}
	s := `<?xml version="1.0" encoding="utf-8" ?>
<body copyright="All data copyright MBTA 2013.">
	<route tag="76" title="76" color="9933cc" oppositeColor="ffffff"
		     latMin="42.3954299" latMax="42.4628099"
		     lonMin="-71.29118" lonMax="-71.14248">
		<stop tag="141" title="Alewife Station Busway"
	        lat="42.3954299" lon="-71.14248" stopId="00141"/>
	  <direction tag="76_0_var0"
	  					 title="Hanscom Civil Airport via Lexington Center"
	             name="Outbound" useForUI="true">
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
</body>`
	body, err := UnmarshalNextbusXml([]byte(s))
	if err != nil {
		t.Error("Unxpected error:", err)
	}
	if body == nil {
		t.Error("nil body not expected")
		return
	}
	compare.ExpectEqual(t.Error, expected, body)
}

// http://webservices.nextbus.com/service/publicXMLFeed?command=schedule&a=mbta
func TestUnmarshalNextbusXml_RouteConfig_Error(t *testing.T) {
	expected := &BodyElement{
		Error: &ErrorElement{
			ElementText: `
  Command would return more routes than the maximum: 100.
 Try specifying batches of routes from "routeList".
`,
			ShouldRetry: false,
		},
	}
	s := `
<?xml version="1.0" encoding="utf-8" ?> 
<body copyright="All data copyright MBTA 2014.">
<Error shouldRetry="false">
  Command would return more routes than the maximum: 100.
 Try specifying batches of routes from "routeList".
</Error>
</body>
`
	body, err := UnmarshalNextbusXml([]byte(s))
	if err != nil {
		t.Error("Unxpected error:", err)
	}
	if body == nil {
		t.Error("nil body not expected")
		return
	}
	compare.ExpectEqual(t.Error, expected, body)
}

// http://webservices.nextbus.com/service/publicXMLFeed?command=schedule&a=mbta&r=76
func TestUnmarshalNextbusXml_Schedule(t *testing.T) {
	expected := &BodyElement{
		Routes: []*RouteElement{
			&RouteElement{
				Tag: "76", Title: "76", ScheduleClass: "20140830",
				ServiceClass: "Friday", Direction: "Inbound",
				SchedHeader: &HeaderElement{
					Stops: []StopElement{
						StopElement{Tag: "85231", ElementText: "Lincoln Lab"},
						StopElement{Tag: "86179_1", ElementText: "Civil Air Terminal"},
						StopElement{Tag: "141_ar", ElementText: "Alewife Station Busway"},
					},
				},
				Trips: []*TrElement{
					&TrElement{
						BlockID: "T350_173",
						Stops: []StopElement{
							StopElement{
								Tag: "85231", EpochTime: 21600000, ElementText: "06:00:00"},
							StopElement{
								Tag: "86179_1", EpochTime: -1, ElementText: "--"},
							StopElement{
								Tag: "141_ar", EpochTime: 23820000, ElementText: "06:37:00"},
						},
					},
				},
			},
		},
	}
	s := `<?xml version="1.0" encoding="utf-8" ?> 
<body copyright="All data copyright MBTA 2014.">
  <route tag="76" title="76" scheduleClass="20140830" serviceClass="Friday"
         direction="Inbound">
    <header>
      <stop tag="85231">Lincoln Lab</stop>
      <stop tag="86179_1">Civil Air Terminal</stop>
      <stop tag="141_ar">Alewife Station Busway</stop>
    </header>
    <tr blockID="T350_173">
      <stop tag="85231" epochTime="21600000">06:00:00</stop>
      <stop tag="86179_1" epochTime="-1">--</stop>
      <stop tag="141_ar" epochTime="23820000">06:37:00</stop>
    </tr>
  </route>
</body>`
	body, err := UnmarshalNextbusXml([]byte(s))
	if err != nil {
		t.Error("Unxpected error:", err)
	}
	if body == nil {
		t.Error("nil body not expected")
		return
	}
	compare.ExpectEqual(t.Error, expected, body)
}
