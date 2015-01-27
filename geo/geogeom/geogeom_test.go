package geogeom

import (
	"github.com/jamessynge/transit_tools/compare"
	"github.com/jamessynge/transit_tools/geo"
	"math"
	"testing"

	//	"github.com/jamessynge/transit_tools/geom"
)

const (
	pi0_4 = 0
	pi1_4 = math.Pi / 4
	pi2_4 = math.Pi * 2 / 4
	pi3_4 = math.Pi * 3 / 4
	pi4_4 = math.Pi
	pi5_4 = math.Pi * 5 / 4
	pi6_4 = math.Pi * 6 / 4
	pi7_4 = math.Pi * 7 / 4
)

func TestHeadingTransform(t *testing.T) {
	tests := []struct {
		ok        bool
		heading   int
		direction float64
	}{
		{false, -45, pi3_4},
		{true, 0, pi2_4},
		{true, 45, pi1_4},
		{true, 90, pi0_4},
		{true, 135, pi7_4},
		{true, 180, pi6_4},
		{true, 225, pi5_4},
		{true, 270, pi4_4},
		{true, 315, pi3_4},
		{true, 360, pi2_4},
	}

	xf := MakeNoOpHeadingTransform()

	for i := range tests {
		test := tests[i]
		dir, err := xf.GeoHeadingToDirection(geo.HeadingInt(test.heading))
		if test.ok {
			if err != nil || dir != test.direction {
				t.Errorf(
					"FromIntegralHeading failure; test case: %v\ndir: %v\nerr: %v",
					test, dir, err)
			}
		} else {
			if err == nil {
				t.Errorf(
					"FromIntegralHeading failure; test case: %v\ndir: %v\nerr: %v",
					test, dir, err)
			}
		}
		dir = xf.ToDirection(float64(test.heading))
		if dir != test.direction {
			t.Errorf("FromHeading failure; test case: %v\ndir: %v", test, dir)
		}
	}
}

func TestDistanceAndHeading(t *testing.T) {
	// MBTA route 76 limits:
	minLat76, maxLat76 := geo.Latitude(42.3954299), geo.Latitude(42.4628099)
	minLon76, maxLon76 := geo.Longitude(-71.29118), geo.Longitude(-71.14248)
	nw := geo.Location{maxLat76, minLon76}
	ne := geo.Location{maxLat76, maxLon76}
	sw := geo.Location{minLat76, minLon76}
	se := geo.Location{minLat76, maxLon76}

	highland := geo.Location{geo.Latitude(42.40961), geo.Longitude(-71.17413)}
	hillcrest := geo.Location{geo.Latitude(42.4104999), geo.Longitude(-71.17731)}
	bellington := geo.Location{geo.Latitude(42.4109099), geo.Longitude(-71.17965)}
	park := geo.Location{geo.Latitude(42.4112499), geo.Longitude(-71.18156)}

	//center := geo.Location{(minLat76+maxLat76)/2, (minLon76+maxLon76)/2}

	tests := []struct {
		name                       string
		loc1, loc2                 geo.Location
		distance, heading, reverse float64
	}{
		{"nw -> ne", nw, ne, 12197.9, 89.95, 270.05},
		{"sw -> se", sw, se, 12211, 89.95, 270.05},
		{"nw -> sw", nw, sw, 7492.3, 180, 0},
		{"ne -> se", ne, se, 7492.3, 180, 0},
		{"highland -> hillcrest", highland, hillcrest, 279.2, 290.759, 110.76},
		{"hillcrest -> bellington", hillcrest, bellington, 197.45, 283.35, 103.35},
		{"highland -> bellington", highland, bellington, 475.68, 287.69, 107.69},
		{"bellington -> park", bellington, park, 161.3, 283.56, 103.55},
	}

	for i := range tests {
		// From loc1 to loc2
		d1, h1 := geo.ToDistanceAndHeading(tests[i].loc1, tests[i].loc2)
		if !compare.NearlyEqual(d1, tests[i].distance) ||
			!compare.NearlyEqual(h1, tests[i].heading) {
			t.Errorf("'to' test case:\n%v\nresults:\n%v  %v", tests[i], d1, h1)
			t.Errorf("distance diff: %v", math.Abs(d1-tests[i].distance))
			t.Errorf(" heading diff: %v", math.Abs(h1-tests[i].heading))
		}
		// From loc2 to loc1
		d2, h2 := geo.ToDistanceAndHeading(tests[i].loc2, tests[i].loc1)
		if !compare.NearlyEqual(d2, tests[i].distance) ||
			!compare.NearlyEqual(h2, tests[i].reverse) {
			t.Errorf("reverse 'to' test case:\n  %v\nreverse results:\n  %v  %v", tests[i], d2, h2)
			t.Errorf("distance diff: %v", math.Abs(d2-tests[i].distance))
			t.Errorf(" heading diff: %v", math.Abs(h2-tests[i].reverse))
		}
		// Distance d1 from loc1 at heading h1
		loc2 := geo.FromDistanceAndHeading(tests[i].loc1, d1, h1)
		if !(compare.NearlyEqual3(
			float64(tests[i].loc2.Lat), float64(loc2.Lat), 0.000001) &&
			compare.NearlyEqual3(
				float64(tests[i].loc2.Lon), float64(loc2.Lon), 0.000001)) {
			t.Errorf("'from' test case:\n%v\nresult:\n%v", tests[i], loc2)
			t.Errorf(" latitude diff: %v", loc2.Lat-tests[i].loc2.Lat)
			t.Errorf("longitude diff: %v", loc2.Lon-tests[i].loc2.Lon)
		}
		// Distance d2 from loc2 at heading h2
		loc1 := geo.FromDistanceAndHeading(tests[i].loc2, d2, h2)
		if !(compare.NearlyEqual3(
			float64(tests[i].loc1.Lat), float64(loc1.Lat), 0.000001) &&
			compare.NearlyEqual3(
				float64(tests[i].loc1.Lon), float64(loc1.Lon), 0.000001)) {
			t.Errorf("'from' test case:\n%v\nresult:\n%v", tests[i], loc1)
			t.Errorf(" latitude diff: %v", loc1.Lat-tests[i].loc1.Lat)
			t.Errorf("longitude diff: %v", loc1.Lon-tests[i].loc1.Lon)
		}
	}
}
