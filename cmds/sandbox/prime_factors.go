package main

import (
	"fmt"
	"math"
	"time"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/util"
)

// From sieve.go in go talks archive...
// Send the sequence of candidate primes to channel 'ch'.
func generate(knownPrimes []int, limit int, ch chan<- int) {
	if len(knownPrimes) < 2 {
		// Send first prime (2) to channel ch, then send all odd integers starting
		// with 3.
		ch <- 2
		for i := 3; i < limit; i += 2 {
			ch <- i // Send 'i' to channel 'ch'.
		}
		if limit%2 == 1 {
			ch <- limit
		}
		close(ch)
		return
	}
	// We have some known primes (e.g. when we've already computed
	// a bunch during a previous call to copyPrimes).
	for _, prime := range knownPrimes {
		ch <- prime // Send 'i' to channel 'ch'.
	}
	for i := knownPrimes[len(knownPrimes)-1] + 2; i < limit; i += 2 {
		ch <- i // Send 'i' to channel 'ch'.
	}
	if limit%2 == 1 {
		ch <- limit
	}
	close(ch)
}

// Copy the values from channel 'src' to channel 'dst',
// removing those divisible by 'prime'.
func filter(src <-chan int, dst chan<- int, prime int) {
	for i := range src { // Loop over values received from 'src'.
		if i%prime != 0 {
			dst <- i // Send 'i' to channel 'dst'.
		}
	}
	close(dst)
}

// The prime sieve.
func sieve(knownPrimes []int, limit int) (primes []int) {
	primes = make([]int, 0, 64)
	ch := make(chan int)
	go generate(knownPrimes, limit, ch)
	for {
		prime, ok := <-ch
		if !ok {
			return
		}
		primes = append(primes, prime)
		ch1 := make(chan int)
		go filter(ch, ch1, prime)
		ch = ch1
	}
}

var gPrimes []int = []int{2, 3, 5}
var gPrimesLimit int = 5

func copyPrimes(limit int) []int {
	if limit == gPrimesLimit {
		glog.Infoln("copy all for limit", limit)
		return append([]int(nil), gPrimes...)
	}
	if limit < gPrimesLimit {
		// Binary search to find index of highest prime to return (i.e. highest
		// element which is <= limit).
		lo, hi := 0, len(gPrimes)-1
		for lo <= hi {
			mid := (lo + hi) / 2
			p := gPrimes[mid]
			if p < limit {
				lo = mid + 1
			} else if p > limit {
				hi = mid - 1
			} else {
				glog.Infoln("copy up through limit", limit)
				return append([]int(nil), gPrimes[0:mid+1]...)
			}
		}
		glog.Infoln("copy up to limit", limit)
		return append([]int(nil), gPrimes[0:lo]...)
	}
	glog.Infoln("generate up to limit", limit)
	gPrimes = sieve(gPrimes, limit)
	gPrimesLimit = limit
	glog.Infoln("copy generated")
	return append([]int(nil), gPrimes...)
}

func testCopyPrimes() {
	primes := copyPrimes(1000)
	glog.Infof("len(primes) == %d\n", len(primes))
	primes = copyPrimes(1000)
	glog.Infof("len(primes) == %d\n", len(primes))
	primes = copyPrimes(500)
	glog.Infof("len(primes) == %d\n", len(primes))
	primes = copyPrimes(1500)
	glog.Infof("len(primes) == %d\n", len(primes))
	primes = copyPrimes(15000)
	glog.Infof("len(primes) == %d\n", len(primes))
	primes = copyPrimes(150000)
	glog.Infof("len(primes) == %d\n", len(primes))
}

//
//
//
//
//
//func ComputePrimeFactors(n uint) (factors []uint) {
//
//
//
//}
//
//func ComputeAllFactors

func gcdMod(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func gcdSub(a, b int) int {
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

func measureGcdFunc(limit int, fn func(a, b int) int) time.Duration {
	start := time.Now()
	for a := 1; a <= limit; a++ {
		for b := 1; b <= a; b++ {
			fn(a, b)
		}
	}
	return time.Since(start)
}

func measureGcdFuncs(limit int) {
	glog.Infoln("measure gcd performance up to limit", limit)
	glog.Infoln("gcdMod duration", measureGcdFunc(limit, gcdMod))
	glog.Infoln("gcdSub duration", measureGcdFunc(limit, gcdSub))
}

func testGcd() {
	for n := 16; n < 65536; n *= 2 {
		measureGcdFuncs(n)
	}
}

func validateGcdFuncs(limit int) {
	low := -1
	for a := 0; a <= limit; a++ {
		for b := 0; b <= limit; b++ {
			s := gcdSub(a, b)
			m := gcdMod(a, b) 
			if s != m {
				glog.Fatalf("gcdSub(%d, %d) -> %d     gcdMod(%d, %d) -> %d", a, b, s, a, b, m)
			}
			sba := gcdSub(b, a)
			if s != sba {
				glog.Fatalf("gcdSub(%d, %d) -> %d     gcdSub(%d, %d) -> %d", a, b, s, b, a, sba)
			}
			mba := gcdMod(b, a)
			if m != mba {
				glog.Fatalf("gcdMod(%d, %d) -> %d     gcdMod(%d, %d) -> %d", a, b, m, b, a, mba)
			}

			if a == 0 {
				if s != b {
					glog.Fatalf("gcd(%d, %d) -> %d   UNEXPECTED", a, b, s)
				}
			} else if b == 0 {
				if s != a {
					glog.Fatalf("gcd(%d, %d) -> %d   UNEXPECTED", a, b, s)
				}
			} else if a == b {
				if s != a {
					glog.Fatalf("gcd(%d, %d) -> %d   UNEXPECTED", a, b, s)
				}
			} else if low < s && s < a && s < b {
				glog.Infof("gcd(%d, %d) -> %d", a, b, s)
				low++
			}
		}
	}
}

func countFactors(v int32) int32 {
	if v <= 3 { return 0 }
	max := (v + 1) / 2
	var cnt int32
	for n := int32(2); n <= max; n++ {
		if v % n == 0 {
			cnt++
		}
	}
	return cnt
}

// Assume we will divide a length only by a small integer, and may further
// subdivide the resulting length again, also by a small integer, and will not
// divide such that the resulting length is less than min.
func countUsefulFactors(v, min int32) int32 {
	smallPrimes := []int32{2,3,5,7}
	var cnt int32
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

func countSpecificFactors(v, min int32, factors map[int32]int32) {
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
func countUsefulSmallPrimeFactors(v, min int32) map[int32]int32 {
	factors := make(map[int32]int32)
	for _, n := range []int32{2,3,5,7,11} {
		if n <= min {
			factors[n] = 0
		}
	}
	countSpecificFactors(v, min, factors)
	return factors
}

func countCommonSmallPrimeFactors(weDistance, snDistance, min int32) map[int32]int32 {
	weFactors := countUsefulSmallPrimeFactors(weDistance, min)
	snFactors := countUsefulSmallPrimeFactors(snDistance, min)
	commonSmallPrimeFactors := make(map[int32]int32)
	for n, weCnt := range weFactors {
		if weCnt < 1 { continue }
		snCnt := snFactors[n]
		if snCnt > weCnt {
			commonSmallPrimeFactors[n] = weCnt
		} else if snCnt > 0 {
			commonSmallPrimeFactors[n] = snCnt
		}
	}
	return commonSmallPrimeFactors
}

type candidatePartition struct {
	snDistance, weDistance, unit int32

	// Fields computed from the above fields and from snMin, weMin and minMinUnit.

	// How much area does this candidate cover beyond the area of snMin*weMin
	extraArea int64
	// What percentage of additional area was added beyond snMin*weMin
	extraAreaPercent int32

	// How many factors does unit have (prime and composite, but not 1 or unit,
	// nor below minMinUnit)? 
	numFactorsOfUnit int32

	// How many squares of size unit*unit fit into snDistance * weDistance
	squares int32

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
//func (cp candidatePartition) MeasureWasteFractionOfSideSquares(snMin, weMin, side int32) float64 {
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

func (cp *candidatePartition) ComputeScore(snMin, weMin, minMinUnit int32) {
	baseArea := int64(snMin) * int64(weMin)
	area := int64(cp.snDistance) * int64(cp.weDistance)
	cp.extraArea = area - baseArea
	cp.extraAreaPercent = int32((area * 100) / baseArea - 100)
	cp.squares = (cp.snDistance / cp.unit) * (cp.weDistance / cp.unit)

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
			float64(smallPrimeFactorCounts[7]) * 0.5 +
			float64(smallPrimeFactorCounts[11]) * 0.4)

	cp.score = pctFactor * largeUnitFactor * spfScore * (math.Sqrt(float64(cp.numFactorsOfUnit)) - 0.9)
}

type candidatePartitions []candidatePartition
func (cs candidatePartitions) LogFirstN(n int, printer func(format string, args... interface{})) {
	for i := 0; i < len(cs) && i < n; i++ {
		c := cs[i]
		printer("unit: %v     factors: %v     extraAreaPercent: %v     score: %v", c.unit, c.numFactorsOfUnit, c.extraAreaPercent, c.score)
	}
}
func (cs candidatePartitions) SortBy(less func(i, j int) bool) {
	swap := func(i, j int) {
		cs[i], cs[j] = cs[j], cs[i]
	}
	util.Sort3(len(cs), less, swap)
}
func (cs candidatePartitions) SortByDescendingUnit() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].unit > cs[j].unit
	})
}
func (cs candidatePartitions) SortByDescendingFactors() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].numFactorsOfUnit > cs[j].numFactorsOfUnit
	})
}
func (cs candidatePartitions) SortByAscendingExtraArea() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].extraArea < cs[j].extraArea
	})
}
func (cs candidatePartitions) SortByDescendingScore() {
	cs.SortBy(func(i, j int) bool {
		return cs[i].score > cs[j].score
	})
}

func generateCandidatePartitionSizes(snMin, snMax, weMin, weMax, minUnit, minMinUnit int32) (
		result candidatePartitions) {
	glog.Infof("generateCandidatePartitionSizes(%v, %v,   %v, %v,   %v)\n",
			snMin, snMax, weMin, weMax, minUnit)

	if snMin > snMax {
		snMin, snMax = snMax, snMin
	}
	if weMin > weMax {
		weMin, weMax = weMax, weMin
	}
	gcdFn := func(a, b int32) int32 {
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
				cp := candidatePartition{
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
		candidate candidatePartition, err error) {
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
	var cs candidatePartitions
	target := minUnit
	for {
		cs = generateCandidatePartitionSizes(
				int32(snMin), int32(math.Ceil(snMax)),
				int32(weMin), int32(math.Ceil(weMax)),
				int32(target + 0.5), int32(minMinUnit))
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
				int32(target + 0.5), int32(nextTarget + 0.5))
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

func testPartitionSize() {
	var cs candidatePartitions
	snDistance, weDistance := 64000, 48000
	for margin := 500; margin <= 4000; margin += 500 {
		cp, err := choosePartitionSize(
				float64(snDistance - margin), float64(snDistance + margin),
				float64(weDistance - margin), float64(weDistance + margin),
				5000, 750)
		if err != nil {
			glog.Error(err)
		} else {
			cs = append(cs, cp)
		}
	}
	for _, cp := range cs {
		glog.Infof("Candidate: %#v", cp)
	}
}
