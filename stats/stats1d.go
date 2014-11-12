package stats

import (
	"fmt"
	"log"
	"math"
	"github.com/jamessynge/transit_tools/util"
)

func Data1DWeightedMedian(source Data1DSource) (value float64, index int) {
	index = -1
	totalWeight := 0.0
	indices := make([]int, source.Len())
	for i := range indices {
		indices[i] = i
		totalWeight += source.Weight(i)
	}
	halfWeight := totalWeight / 2
	less := func(i, j int) bool {
		u, v := indices[i], indices[j]
		return source.Value(u) < source.Value(v)
	}
	swap := func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	}
	util.Sort3(len(indices), less, swap)
	cumulativeWeight := 0.0
	for i := range indices {
		u := indices[i]
		w := source.Weight(u)
		cumulativeWeight += w
		if cumulativeWeight < halfWeight {
			continue
		}
		value = source.Value(u)
		index = i
		return
	}
	// How could we possibly get here!?
	log.Panicf("totalWeight=%g, halfWeight=%g, cumulativeWeight=%g, len=%d",
		totalWeight, halfWeight, cumulativeWeight, source.Len())
	return
}

// Based on https://en.wikipedia.org/wiki/Compensated_summation
type KahanSum struct {
	Sum float64
	// A running compensation for lost low-order bits.
	c float64
}

func (p *KahanSum) Add(v float64) {
	y := v - p.c          // Initially y = v because c is zero.
	t := p.Sum + y        // Alas, sum is big, y small, so low-order digits of y are lost.
	p.c = (t - p.Sum) - y // (t - sum) recovers the high-order part of y; subtracting y recovers -(low part of y)
	p.Sum = t             // Algebraically, c should always be zero. Beware overly-aggressive optimising compilers!
}

type Running1DStats struct {
	count            int64
	min, max         float64
	sum, sum_squares KahanSum
}

func (p *Running1DStats) String() string {
	return fmt.Sprintf(
		"{Mean: %v; StdDev: %v; Range: %v to %v; Count: %d}",
		p.Mean(), p.StandardDeviation(), p.min, p.max, p.count)
	//	return fmt.Sprintf(
	//			"{cnt:%d,min:%v,max:%v,sum:%v,sum_squares:%v,mean:%v,stddev:%v}",
	//			p.count, p.min, p.max, p.sum.Sum, p.sum_squares.Sum,
	//			p.Mean(), p.StandardDeviation())
}
func (p *Running1DStats) Add(v float64) {
	if p.count == 0 {
		p.min = v
		p.max = v
	} else {
		p.min = math.Min(p.min, v)
		p.max = math.Max(p.max, v)
	}
	p.sum.Add(v)
	p.sum_squares.Add(v * v)
	p.count++
}
func (p *Running1DStats) Count() int64 {
	return p.count
}
func (p *Running1DStats) Min() float64 {
	return p.min
}
func (p *Running1DStats) Max() float64 {
	return p.max
}
func (p *Running1DStats) Mean() float64 {
	return p.sum.Sum / float64(p.count)
}
func (p *Running1DStats) Variance() float64 {
	mean := p.Mean()
	return p.sum_squares.Sum/float64(p.count) - mean*mean
}
func (p *Running1DStats) StandardDeviation() float64 {
	return math.Sqrt(p.Variance())
}
