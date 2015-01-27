package geo

import (
	"fmt"
	"strconv"
)

// Valid NextBus headings are in the range 0 to 360 (inclusive, for some reason
// of 360; perhaps rounding errors?).  Negative values indicate that the heading
// is unavailable; usually I see -1 in that case, but occasionally see -2.
// 0 and 360 appear to be north, 90 east, 180 south, 270 west.
type HeadingInt int
type HeadingF float64

const (
	minHeading = -2
	maxHeading = 360
)

func HeadingFromInt(v int) (heading HeadingInt, err error) {
	if minHeading <= v && v <= maxHeading {
		return HeadingInt(v), nil
	}
	return HeadingInt(-1), fmt.Errorf("Heading out of expected range: %d", v)
}

func ParseHeading(s string) (heading HeadingInt, err error) {
	v, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		return HeadingInt(-1), err
	}
	if minHeading <= v && v <= maxHeading {
		return HeadingInt(int(v)), nil
	}
	return HeadingInt(-1), fmt.Errorf("Heading out of expected range: %q", s)
}

func (h HeadingInt) IsValid() bool {
	return 0 <= h && h <= maxHeading
}

//var headingPopulation int
//var headingCensus [361]int
//		headingCensus[v]++
//		headingPopulation++
//		if headingPopulation > 500000 {
//			pop := float64(headingPopulation)
//			headingPopulation = 0
//			for i, c := range headingCensus {
//				frac := float64(c) / pop * 361
////				pct := float64(c) / pop * 100
//				log.Printf("HEADING CENSUS: %ddegrees = %v fraction of expected", i, frac)
//				headingCensus[i] = 0
//			}
//		}

func (h HeadingF) ToRadians() float64 {
	return toRadians(float64(h))
}
