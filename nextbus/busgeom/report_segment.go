package busgeom

import (
	"geom"
)

// Data about relationship between a segment and a report.
type ReportNearSegment struct {
	Distance        float64
	AngleBetween    float64
	NearestPoint    geom.Point
	IsPerpendicular bool
}

func MakeReportNearSegment(
	report *Report, seg *PathSegment) (rns *ReportNearSegment) {
	rns = &ReportNearSegment{
		AngleBetween: seg.Direction.AngleBetween(report.Direction),
	}
	rns.NearestPoint, rns.IsPerpendicular = seg.ClosestPointTo(report.Point)
	rns.Distance = rns.NearestPoint.Distance(report.Point)
	return
}

func LinkReportAndSegment(
	report *Report, seg *PathSegment) (rns *ReportNearSegment) {
	rns = MakeReportNearSegment(report, seg)

	if report.nearSegs == nil {
		report.nearSegs = make(map[*PathSegment]*ReportNearSegment)
	}
	report.nearSegs[seg] = rns

	if seg.nearReports == nil {
		seg.nearReports = make(map[*Report]*ReportNearSegment)
	}
	seg.nearReports[report] = rns

	return rns
}
