package main

import (
	"compare"
	"fmt"
	"geo"
	"nextbus"
	"time"
)

func Dummy1(x geo.Location, y nextbus.VehicleLocation) {

}

func main() {
	s := `<?xml version="1.0" encoding="utf-8" ?>
<body copyright="All data copyright MBTA 2012.">
<Error shouldRetry="false">
  last time "t" parameter must be specified in query string
</Error>
<vehicle id="0199" routeTag="64" dirTag="64_1_var0" lat="42.3685977" lon="-71.0991791" secsSinceReport="20" predictable="true" heading="118" speedKmHr="0.0"/>
<vehicle id="0877" routeTag="451" dirTag="451_1_var0" lat="42.5513283" lon="-70.878608" secsSinceReport="35" predictable="true" heading="160" speedKmHr="0.0"/>
<lastTime time="1350562779906"/>
</body>`


	unmarshalNextbusXml


	locations, err := nextbus.ParseXmlVehicleLocations(s)
	if err != nil {
		fmt.Printf("error returned, not locations: %v\n", err)
	} else if len(locations) != 2 {
		fmt.Printf("2 Locations not expected, not %d\n", len(locations))
	} else {
		expected0 := &nextbus.VehicleLocation{
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
		fmt.Printf("expected0: %v\n", expected0)
		compare.ExpectEqual(
			func(args ...interface{}) { fmt.Println(args...) },
			expected0, locations[0])
	}
}
