package nextbus

import (
	"testing"
)

func TestParseRouteXml(t *testing.T) {

	s := `<?xml version="1.0" encoding="utf-8" ?>
<body copyright="All data copyright MBTA 2013.">
<route tag="76" title="76" color="9933cc" oppositeColor="ffffff"
  latMin="42.3954299" latMax="42.4628099" lonMin="-71.29118" lonMax="-71.14248">
<stop tag="141" title="Alewife Station Busway" lat="42.3954299" lon="-71.14248" stopId="00141"/>
<stop tag="2480" title="Rt 2 Westbound Pedestrian Bridge" lat="42.3991199" lon="-71.1462" stopId="02480"/>
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
</body>`

	body, err := UnmarshalNextbusXml([]byte(s))

	if err != nil {
		t.Fatal(err)
	} else if body == nil {
		t.Fatal("No body returned")
	}

	//	t.Fatalf("body: %#v", body)
}
