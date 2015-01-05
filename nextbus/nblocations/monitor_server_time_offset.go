package nblocations

import (
	"github.com/jamessynge/transit_tools/stats"
)

type TimeOffsetMonitor struct {
	samples *stats.SlidingWindowTimeDurationSource
}

func NewTimeOffsetMonitor(sampleLimit int) *TimeOffsetMonitor {
	return &TimeOffsetMonitor{
		samples: stats.NewSlidingWindowTimeDurationSource(sampleLimit),
	}
}

func (p *TimeOffsetMonitor) Update(vlr *VehicleLocationsResponse) {
	// Plan:
	// 		Compute stats 2d based on previous samples
	// 		Fit line to previous samples (linear least squares, not orthogonal
	//			regression, since the x and y axis values are not independent)
	//		Compute expected range of values at new time (e.g. what is the
	//			standard deviation of values?)
	//		Compute difference between between line and new point; how does that
	//			compare to expected range of values?

	// Also, consider how the Date header relates to the secsSinceReport
	// and time attributes in vehicle responses.
}
