// Metric space (i.e. not lat-long) related code for working with bus route
// paths and vehicle locations.

package busgeom

import (
	"geom"
	"log"
	"math"
	"stats"
)

const (
	halfPi = math.Pi / 2
	pi     = math.Pi
	twoPi  = math.Pi * 2
)

type WeightFunction3 func(seg *PathSegment,
	report *Report,
	rns *ReportNearSegment) float64

type RouteDirectionStats struct {
	dirTag2Stats map[string]*stats.Running1DStats
}

func MakeRouteDirectionStats() *RouteDirectionStats {
	return &RouteDirectionStats{
		dirTag2Stats: make(map[string]*stats.Running1DStats),
	}
}
func (p *RouteDirectionStats) AddReport(report *Report) {
	_, rns := report.ClosestNearPerpendicularSegment()
	if rns == nil {
		return
	}
	rs, ok := p.dirTag2Stats[report.DirTag]
	if !ok {
		log.Printf("Adding stats for %s", report.DirTag)
		rs = &stats.Running1DStats{}
		p.dirTag2Stats[report.DirTag] = rs
	}
	rs.Add(rns.AngleBetween)
}

func (p *RouteDirectionStats) PrintStats(
	printf func(format string, v ...interface{})) {
	printf("----------------- RouteDirectionStats -----------------\n")
	for dirTag, rs := range p.dirTag2Stats {
		printf("%-20s: %v", dirTag, rs)
	}
	printf("-------------------------------------------------------\n")

}

// Support for figuring out which route directions (e.g. 76_0_var0) are
// travelling this path in the correct direction.

func MeasureRouteDirections(
	allReports []*Report,
	maxDistance float64) (dirTag2Stats map[string]*stats.Running1DStats) {
	log.Printf("MeasureRouteDirections =================================\n")
	log.Printf("len(allReports) == %d", len(allReports))

	rds := MakeRouteDirectionStats()

	for _, report := range allReports {
		rds.AddReport(report)
	}

	rds.PrintStats(log.Printf)
	return rds.dirTag2Stats
}

func MeasureRouteDirectionsOrig(
	qt geom.QuadTree, // QuadTree with all the segments of the path.
	maxDistance float64,
	allReports []*Report) (dirTag2Data map[string]*stats.Running1DStats) {
	log.Printf("MeasureRouteDirections =================================\n")
	log.Printf("len(allReports) == %d", len(allReports))

	nothingClose := 0
	noHeading := 0

	dirTag2Data = make(map[string]*stats.Running1DStats)
	for _, report := range allReports {
		segs := NearbyPathSegments(qt, report.Point, maxDistance)
		closestSegment, _, _ := NearestPathSegmentInSlice(report.Point, segs)
		if closestSegment == nil {
			nothingClose++
			continue
		}
		angle, ok := closestSegment.AngleBetween(report)
		if !ok {
			noHeading++
			if noHeading < 20 {
				log.Printf("No direction: %v", report)
			}
			continue
		}
		rs, ok := dirTag2Data[report.DirTag]
		if !ok {
			log.Printf("Adding stats for %s", report.DirTag)
			rs = &stats.Running1DStats{}
			dirTag2Data[report.DirTag] = rs
		}
		rs.Add(angle)
	}

	log.Printf("nothingClose == %d", nothingClose)
	log.Printf("noHeading == %d", noHeading)

	log.Printf("-------------------------------------------------------\n")
	for dirTag, rs := range dirTag2Data {
		log.Printf("%-20s: %v", dirTag, rs)
	}
	log.Printf("-------------------------------------------------------\n")
	return
}

//func TrustedReportsNearSegment(
//		seg *PathSegment, allReports []*Report,
//		dirTagsToTrust map[string]bool) (trustedReports []*Report) {
//	for _, report := range allReports {
//		if !dirTagsToTrust[report.DirTag] { continue }
//		for _, ns := range report.nearSegs {
//			if ns.Segment == seg {
////				if ns.AngleBetween > (pi * 3 / 4) { continue }
//				trustedReports = append(trustedReports, report)
//				break
//			}
//		}
//	}
//	return
//}

func EstimatedSegmentDirection(
	seg *PathSegment,
	dirTagsToTrust map[string]bool) (dir geom.Direction, ok bool) {
	weightFunc := func(
		seg *PathSegment, report *Report, rns *ReportNearSegment) float64 {
		if !dirTagsToTrust[report.DirTag] {
			return 0
		}
		if rns.Distance > 10 {
			return 1 / (rns.Distance - 9)
		} else {
			return 1
		}
	}
	report2Weight := make(map[*Report]float64)
	for report, rns := range seg.nearReports {
		weight := weightFunc(seg, report, rns)
		if weight > 0 {
			report2Weight[report] = weight
		}
	}
	return MedianWeightedReportDirection(report2Weight)
}
