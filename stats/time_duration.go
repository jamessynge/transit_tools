package stats

import (
	"time"

	"github.com/jamessynge/transit_tools/util"
)

////////////////////////////////////////////////////////////////////////////////

// A Data2DSource, for equally weighted samples of Time (unix seconds)
// vs. Duration (seconds)
type SlidingWindowTimeDurationSource struct {
	ds *SlidingWindowData2DSource
}

func NewSlidingWindowTimeDurationSource(
	sampleLimit int) *SlidingWindowTimeDurationSource {
	return &SlidingWindowTimeDurationSource{
		ds: NewSlidingWindowData2DSource(sampleLimit),
	}
}

func (p *SlidingWindowTimeDurationSource) AddSample(
	t time.Time, d time.Duration) {
	p.ds.AddSample(util.TimeToUnixSeconds(t), d.Seconds(), 1.0)
}

func (p *SlidingWindowTimeDurationSource) SampleLimit() int {
	return p.ds.SampleLimit()
}

func (p *SlidingWindowTimeDurationSource) Len() int {
	return p.ds.Len()
}

func (p *SlidingWindowTimeDurationSource) X(n int) float64 {
	return p.ds.X(n)
}

func (p *SlidingWindowTimeDurationSource) Y(n int) float64 {
	return p.ds.X(n)
}

func (p *SlidingWindowTimeDurationSource) Weight(n int) float64 {
	return p.ds.X(n)
}
