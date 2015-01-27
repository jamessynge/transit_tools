package nblocations

import (
	"fmt"
	"math"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/util"
)

// Alternate approach to creating partitioners: aim for creating as many
// square-ish partitions as possible, such that the partitions cover at
// least some minimum region area (i.e. no smaller than 0.5 sq km), and
// that have some minimum fraction of the records in the samples. Of course,
// I then need code for representing those partitions and performing the
// partitioning process, which could conceivably be entirely different from
// the first version.
//

func gcdInt(a, b int) int {
	if a == 0 {
		return b
	}
	for b != 0 {
		if a > b {
			a -= b
		} else if a < b {
			b -= a
		} else {
			return a
		}
	}
	return a
}

type Factors struct {
	f map[int]int
}
func (p *Factors) makeNewMap() {
	p.f = make(map[int]int)
}
func (p *Factors) ensureMapExists() {
	if p.f == nil {
		p.makeNewMap()
	}
}
func (p *Factors) CountKeysIn(v, min int) {
	for n := range p.f {
		cnt := 0
		limit := min * n
		for v >= limit && v % n == 0 {
			v /= n
			cnt++
		}
		p.f[n] = cnt
		if v < min {
			return
		}
	}
}
func (p *Factors) CountTheseFactorsIn(v, min int, factors ...int) {
	p.makeNewMap()
	for _, n := range factors {
		cnt := 0
		limit := min * n
		for v >= limit && v % n == 0 {
			v /= n
			cnt++
		}
		p.f[n] = cnt
		if v <= min {
			return
		}
	}
}
func (f1 Factors) CommonFactors(f2 Factors) (f3 Factors) {
	f3.makeNewMap()
	for n, cnt1 := range f1.f {
		v := 0
		if cnt2, ok := f2.f[n]; ok {
			if cnt1 > cnt2 {
				v = cnt2
			} else {
				v = cnt1
			} 
		}
		if v > 0 {
			f3.f[n] = v
		}
	}
	return
}

// Assume we will divide a length only by a small prime, and may further
// subdivide the resulting length again, also by a small prime, and will not
// divide such that the resulting length is less than min.
func countUsefulFactors(v, min int) int {
	smallPrimes := []int{2,3,5,7}
	var cnt int
	for _, n := range smallPrimes {
		for v % n == 0 {
			v /= n
			if v < min {
				return cnt
			}
			cnt++
		}
	}
	return cnt
}

func countSpecificFactors(v, min int, factors map[int]int) {
	for n := range factors {
		for v % n == 0 {
			v /= n
			if v < min {
				return
			}
			factors[n]++
		}
	}
}

// Assume we will divide a length only by a small integer, and may further
// subdivide the resulting length again, also by a small integer, and will not
// divide such that the resulting length is less than min.
func countUsefulSmallPrimeFactors(v, min int) map[int]int {
	factors := make(map[int]int)
	for _, n := range []int{2,3,5,7,11} {
		if n <= min {
			factors[n] = 0
		}
	}
	countSpecificFactors(v, min, factors)
	return factors
}

type candidateRootPartition struct {
	snDistance, weDistance, unit int

	// Fields computed from the above fields and from snMin, weMin and minMinUnit.

	// How many squares of size unit*unit fit into snDistance * weDistance
	squares int64

	// How much area does this candidate cover beyond the area of snMin*weMin
	extraArea int64
	// What percentage of additional area was added beyond snMin*weMin
	extraAreaPercent int

	// How many small prime factors does unit have?
	numFactorsOfUnit int

	// Number of times the small primes (e.g. 2, 3, 5) go into unit.
	smallPrimeFactorCounts map[int]int

	score float64
}

// Don't need this: the smallest fraction will be with the largest side.
//
//	// How well do we fill the side squares? Depends upon how much we divide at
//	// the first level; could be by cp.unit, but also by some other amount (i.e.
//	// if smallestSide / cp.unit == 2, and but we can also divide the sides by
//	// 3 and 5, respectively, for example and get squares, we might want to do
//	// that so as to decrease the area of the edge squares).
//	
//// If we were to divide into squares of size side*side, how
//// well would we fill out the edges? We assume that the margin around snMin*weMin
//// is the same all the way around.
//func (cp candidateRootPartition) MeasureWasteFractionOfSideSquares(snMin, weMin, side int32) float64 {
//	if cp.snDistance % side != 0 { glog.Fatalf("%v %% %v != 0", cp.snDistance, side) }
//	if cp.weDistance % side != 0 { glog.Fatalf("%v %% %v != 0", cp.weDistance, side) }
//
//	snDivisions := cp.snDistance / side
//	halfSnExtra := (cp.snDistance - snMin) / 2
//	snEdgeUsed := side - halfSnExtra
//
//	weDivisions := cp.weDistance / side
//	halfWeExtra := (cp.weDistance - weMin) / 2
//	weEdgeUsed := side - halfWeExtra
//
//
//}

func (cp *candidateRootPartition) ComputeScore(snMin, weMin, minMinUnit int) {
	baseArea := int64(snMin) * int64(weMin)
	area := int64(cp.snDistance) * int64(cp.weDistance)
	cp.extraArea = area - baseArea
	cp.extraAreaPercent = int((area * 100) / baseArea - 100)
	cp.squares = int64(cp.snDistance / cp.unit) * int64(cp.weDistance / cp.unit)

	cp.numFactorsOfUnit = countUsefulFactors(cp.unit, minMinUnit)

	// Now compute score...  We prefer candidates that:
	//  * have small extra area so that we don't have lots of relatively
	//    vacant squares around the edge.
	//  * have a large number of useful factors so that we can partition it
	//    many times. Probably best if we have several factors of 3 and 5
	//    so that we can have large exterior squares that are less populous,
	//    and small inner squares that are the result of many divisions.

	// Factor that goes down as extraAreaPercent goes up; quantized because
	// we start with extraAreaPercent, an integer, so we're not super fussy.
	pctFactor := 100.0 / float64(100 + cp.extraAreaPercent * 6)

	// How many times does unit go into SN and WE distances? The more, the lower
	// this factor (i.e. we prefer unit to be larger). Because SN and WE distances
	// are also variable, this may be the same for many candidates.
	squares := (cp.snDistance / cp.unit) * (cp.weDistance / cp.unit)
	largeUnitFactor := 1.0 / math.Sqrt(float64(squares))

	// Prefer dividing into 5 and 3, then 2, then 7 and 11.
	smallPrimeFactorCounts := countUsefulSmallPrimeFactors(cp.unit, minMinUnit)
	spfScore := math.Sqrt(
			float64(smallPrimeFactorCounts[5]) * 1 +
			float64(smallPrimeFactorCounts[3]) * 0.9 +
			float64(smallPrimeFactorCounts[2]) * 0.7 +
			float64(smallPrimeFactorCounts[7]) * 0.2 +
			float64(smallPrimeFactorCounts[11]) * 0.1)

	cp.score = pctFactor * largeUnitFactor * spfScore * (math.Sqrt(float64(cp.numFactorsOfUnit)) - 0.9)
}

type candidateRootPartitions []candidateRootPartition
func (cs candidateRootPartitions) LogFirstN(n int, printer func(format string, args... interface{})) {
	for i := 0; i < len(cs) && i < n; i++ {
		c := cs[i]
		printer("unit: %v     factors: %v     extraAreaPercent: %v     score: %v", c.unit, c.numFactorsOfUnit, c.extraAreaPercent, c.score)
	}
}
func (cs candidateRootPartitions) SortBy(less func(i, j int) bool) {
	swap := func(i, j int) {
		cs[i], cs[j] = cs[j], cs[i]
	}
	util.Sort3(len(cs), less, swap)
}
func (cs candidateRootPartitions) SortByDescendingUnit() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].unit > cs[j].unit
	})
}
func (cs candidateRootPartitions) SortByDescendingFactors() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].numFactorsOfUnit > cs[j].numFactorsOfUnit
	})
}
func (cs candidateRootPartitions) SortByAscendingExtraArea() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].extraArea < cs[j].extraArea
	})
}
func (cs candidateRootPartitions) SortByDescendingScore() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].score > cs[j].score
	})
}

func generateCandidatePartitionSizes(snMin, snMax, weMin, weMax, minUnit, minMinUnit int) (
		result candidateRootPartitions) {
	glog.Infof("generateCandidatePartitionSizes(%v, %v,   %v, %v,   %v)\n",
			snMin, snMax, weMin, weMax, minUnit)

	if snMin > snMax {
		snMin, snMax = snMax, snMin
	}
	if weMin > weMax {
		weMin, weMax = weMax, weMin
	}
	gcdFn := func(a, b int) int {
		if a == 0 {
			return b
		}
		for b != 0 {
			if a > b {
				a -= b
			} else if a < b {
				b -= a
			} else {
				return a
			}
		}
		return a
	}
	for sn := snMin; sn <= snMax; sn++ {
		for we := weMin; we <= weMax; we++ {
			gcd := gcdFn(sn, we)
			if gcd >= minUnit {
				cp := candidateRootPartition{
					snDistance: sn,
					weDistance: we,
					unit: gcd,
				}
				cp.ComputeScore(snMin, weMin, minMinUnit)
				result = append(result, cp)
			}
		}
	}
	if len(result) == 0 {
		glog.Warningf(`Unable to find candidate partition size for these constraints
      sn: %v to %v
      we: %v to %v
      minUnit: %v`, snMin, snMax, weMin, weMax, minUnit)
	}
	return
}

func choosePartitionSize(snMin, snMax, weMin, weMax, minUnit, minMinUnit float64) (
		candidate candidateRootPartition, err error) {
	if snMin > snMax {
		snMin, snMax = snMax, snMin
	}
	if weMin > weMax {
		weMin, weMax = weMax, weMin
	}
	if snMax - snMin < minMinUnit {
		glog.Warningf(`South-North range is very small, unlikely to find candidate partition size for these constraints:
      sn: %v to %v  (range %v)
      minUnit: %v`, snMin, snMax, snMax - snMin, minUnit)
  }
	if weMax - weMin < minMinUnit {
		glog.Warningf(`West-East range is very small, unlikely to find candidate partition size for these constraints:
      sn: %v to %v  (range %v)
      minUnit: %v`, weMin, weMax, weMax - weMin, minUnit)
  }
	var cs candidateRootPartitions
	target := minUnit
	for {
		cs = generateCandidatePartitionSizes(
				int(snMin), int(math.Ceil(snMax)),
				int(weMin), int(math.Ceil(weMax)),
				int(target + 0.5), int(minMinUnit))
		if len(cs) > 0 { break }
		nextTarget := target / 2
		if nextTarget < minMinUnit && target > minMinUnit {
			nextTarget = minMinUnit
		}
		if nextTarget < minMinUnit {
			err = fmt.Errorf(`Unable to find candidate partition size for these constraints
      sn: %v to %v
      we: %v to %v
      minUnit: %v
      minMinUnit: %v`, snMin, snMax, weMin, weMax, minUnit, minMinUnit)
			return
		}
		glog.Infof("Reducing target from %v to %v",
				int(target + 0.5), int(nextTarget + 0.5))
		target = nextTarget
	}
	glog.Infof("Found %d candidates", len(cs))
	// Want to choose a candidate that has a large unit size, large number of
	// factors (i.e. so that we can divide it many times), and a small extra
	// area.

	glog.Info("Candidates sorted by descending unit")
	cs.SortByDescendingUnit()
	cs.LogFirstN(10, glog.Infof)

	glog.Info("Candidates sorted by ascending extra area")
	cs.SortByAscendingExtraArea()
	cs.LogFirstN(10, glog.Infof)

	glog.Info("Candidates sorted by descending number of factors of unit")
	cs.SortByDescendingFactors()
	cs.LogFirstN(10, glog.Infof)

	glog.Info("Candidates sorted by descending score")
	cs.SortByDescendingScore()
	cs.LogFirstN(10, glog.Infof)

	return cs[0], nil
}









// Still want to start by creating 5 partitions, 4 for the rest of the world,
// one for the transit region; alternately, create 1 for the transit region with
// a large margin, and drop all outliers on the floor.

type Meters float64
type SquareMeters float64

type cp2Region struct {
	parent *cp2Region

	geo.Rect

	// Distance through the center of the region.
	snDistance Meters
	weDistance Meters

	// Approximate, not counting for curvature, but good
	// enough for small latitude changes.
	area SquareMeters

	samplesWestToEast []geo.Location
	samplesSouthToNorth []geo.Location

	quadrants [4]*cp2Region
}

func NewCp2Region(west, east geo.Longitude, south, north geo.Latitude) *cp2Region {
	p := &cp2Region{
		Rect: geo.Rect{
			West: west,
			East: east,
			South: south,
			North: north,
		},
	}
	sn, we, area := geo.MeasureCentralAxesAndArea(west, east, south, north)
	p.snDistance = Meters(sn)
	p.weDistance = Meters(we)
	p.area = SquareMeters(area)
	return p
}

// Represents the entire globe as 5 sub-regions, the transit region and 4
// exterior regions (i.e. the rest of the globe outside of the transit
// region), arranged like this, on the assumption that the transit region
// doesn't abut or overlap longitude 180°, and doesn't get near the poles.
//
//                 |
//                 |
//                 |        ne
//                 |
//                 |
//        nw       +---------+------------    
//                 |         |                 
//                 | Transit |                 
//                 |  Region |                 
//                 |         |                 
//                 |         |                 
//      -----------+---------+    se
//                           |
//                           |                 
//            sw             |                  
//                           |                  
//                           |                  

type transitSubRegion struct {
	globe *globalRegion
	// Null if this is the root of the transit regions.
	parent *transitSubRegion

	geo.Rect

	// Distance through the center of the region.
	snDistance Meters
	weDistance Meters

	// Approximate, not counting for curvature, but good
	// enough for small latitude changes.
	area SquareMeters

	samplesWestToEast []geo.Location
	samplesSouthToNorth []geo.Location

	candidateFactors map[int]int



	subRegions [][]*transitSubRegion
}




type globalRegion struct {
	transitRegion *transitSubRegion

	nw, ne, se, sw geo.Rect
}










type cp2State struct {
	minimumSide Meters
	minimumArea SquareMeters
	minimumSampleCount int





	root *cp2Region

	// Exterior regions (i.e. the rest of the globe outside of the transit
	// region).  Arranged like this, on the assumption that the transit region
	// doesn't abut or overlap longitude 180° (East or West, same thing),
	// and doesn't get near the poles.
	//
	//                 |
	//                 |
	//                 |        ne
	//                 |
	//                 |
	//        nw       +---------+------------    
	//                 |         |                 
	//                 | Transit |                 
	//                 |  Region |                 
	//                 |         |                 
	//                 |         |                 
	//      -----------+---------+    se
	//                           |
	//                           |                 
	//            sw             |                  
	//                           |                  
	//                           |                  

	nw, ne, se, sw geo.Rect
}

//func expandGeoRect







//func newCp2State(west, east geo.Longitude, south, north geo.Latitude



















////////////////////////////////////////////////////////////////////////////////

func CreateAgencyPartitioner2(agency string, samples []geo.Location,
	levelLimit uint, cutsPerLevel uint) *AgencyPartitioner {
	if levelLimit < 2 {
		glog.Fatalf("Too few levels (%d)", levelLimit)
	}
	if cutsPerLevel < 1 {
		glog.Fatalf("Too few cuts per level (%d)", cutsPerLevel)
	}
	var partitionsPerLevel int64 = int64(cutsPerLevel) + 1
	var totalLeaves int64 = 1
	for n := uint(0); n < (levelLimit - 2); n++ {
		totalLeaves *= partitionsPerLevel
	}
	totalLeaves += 4 // For the two special leaves at each of the first two levels.
	minSamples := totalLeaves * 100
	if int64(len(samples)) < minSamples {
		glog.Fatalf(
			"Too few location samples (%d); want at least %d samples given %d leaves in the partitioning",
			len(samples), minSamples, totalLeaves)
	}

	// Start the partitioning in the direction with the greatest distance.
	snLocations := append([]geo.Location(nil), samples...)
	geo.SortSouthToNorth(snLocations)

	southern, northern := getLocationsNearExtrema(snLocations)
	glog.Infof("southern position: %s", southern)
	glog.Infof("northern position: %s", northern)

	weLocations := samples
	samples = nil
	geo.SortWestToEast(weLocations)
	western, eastern := getLocationsNearExtrema(weLocations)
	glog.Infof("western position: %s", western)
	glog.Infof("eastern position: %s", eastern)

	var south, north, west, east geo.Location
	south.Lat = southern.Lat
	north.Lat = northern.Lat
	west.Lon = western.Lon
	east.Lon = eastern.Lon

	// Now use the median latitude and longitude for computing the
	// longitude and latitude distances, respectively.
	south.Lon = weLocations[len(weLocations)/2].Lon
	north.Lon = south.Lon
	west.Lat = snLocations[len(snLocations)/2].Lat
	east.Lat = west.Lat

	glog.Infof("south: %s", south)
	glog.Infof("north: %s", north)
	glog.Infof("west: %s", west)
	glog.Infof("east: %s", east)

	// Measure the distance.
	snDistance, _ := geo.ToDistanceAndHeading(south, north)
	weDistance, _ := geo.ToDistanceAndHeading(west, east)

	glog.Infof("snDistance: %v", snDistance)
	glog.Infof("weDistance: %v", weDistance)

	// TODO add func createRootPartitioners to create the 5 partitioners for
	// the two root levels.

	result := &AgencyPartitioner{Agency: agency}
	if weDistance >= snDistance {
		result.RootPartitioner = CreateWestEastPartitioners(
			-180, 180, -90, 90,
			0, levelLimit, int(cutsPerLevel),
			weLocations, true)
	} else {
		result.RootPartitioner = CreateSouthNorthPartitioners(
			-180, 180, -90, 90,
			0, levelLimit, int(cutsPerLevel),
			snLocations, false)
	}

	return result
}
