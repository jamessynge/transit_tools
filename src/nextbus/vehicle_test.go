package nextbus

import (
	"fmt"
	"testing"
)

func TestUnmarshalVehicleLocations(t *testing.T) {
	fmt.Println()
	fmt.Println()
	s := `<?xml version="1.0" encoding="utf-8" ?>
<body copyright="All data copyright MBTA 2012.">
<Error shouldRetry="false">
  last time "t" parameter must be specified in query string
</Error>
<vehicle id="0199" routeTag="64" dirTag="64_1_var0" lat="42.3685977" lon="-71.0991791" secsSinceReport="20" predictable="true" heading="118" speedKmHr="0.0"/>
<vehicle id="0877" routeTag="451" dirTag="451_1_var0" lat="42.5513283" lon="-70.878608" secsSinceReport="35" predictable="true" heading="160" speedKmHr="0.0"/>
<lastTime time="1350562779906"/>
</body>`

	body, err := UnmarshalNextbusXml([]byte(s))

	if err != nil {
		t.Fatal(err)
	} else if body == nil {
		t.Fatal("No body returned")
	}

	fmt.Printf("body: %#v\n\n", body)
	fmt.Printf("body: %#v\n\n", body.Error)
	for i, elem := range body.Vehicles {
		fmt.Printf("body.Vehicles[%d]: %#v\n\n", i, elem)
	}
}
