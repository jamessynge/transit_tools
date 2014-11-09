package busgeom

import (
	//	"flag"
	"log"
	"nextbus"
	//	"path/filepath"
	"geo/geogeom"
	"geom"
	"util"
	//	"geo"
	"math"
	//	"fmt"
	"io"
	//	"fit"
	"stats"
)

var (
	kNegativeInfinity = math.Inf(-1)

	debug_NoDir     = 0
	debug_NoHeading = 0
	unknownDirTags  map[string]bool
)

type NearbyPathSegment struct {
	Segment         *PathSegment
	Distance        float64
	AngleBetween    float64
	NearestPoint    geom.Point
	IsPerpendicular bool
}

type Report struct {
	geom.Point
	geom.Direction

	DirTag   string
	nearSegs map[*PathSegment]*ReportNearSegment
}

// CAN'T SORT, nearSegs IS NOW A MAP.
//func (p *Report) SortNearSegments() {
//	// Put the segments in order, closest to furthest.
//	lessFunc := func(i, j int) bool {
//		return p.nearSegs[i].Distance < p.nearSegs[j].Distance
//	}
//	swapFunc := func(i, j int) {
//		p.nearSegs[i], p.nearSegs[j] = p.nearSegs[j], p.nearSegs[i]
//	}
//	util.Sort3(len(p.nearSegs), lessFunc, swapFunc)
//}

func (p *Report) ClearNearSegments() {
	p.nearSegs = make(map[*PathSegment]*ReportNearSegment)
}
func (p *Report) ClosestNearPerpendicularSegment() (closestSegment *PathSegment, closestRNS *ReportNearSegment) {
	dist := math.MaxFloat64
	for seg, rns := range p.nearSegs {
		if !rns.IsPerpendicular {
			continue
		}
		if rns.Distance < dist {
			dist = rns.Distance
			closestSegment = seg
			closestRNS = rns
		}
	}
	return
}

func ClearNearSegments(reports []*Report) {
	for _, report := range reports {
		report.ClearNearSegments()
	}
}

func NewReport(vl *nextbus.VehicleLocation, agency *nextbus.Agency,
	xf geogeom.CoordTransform) (report *Report) {
	report = &Report{
		//		loc: vl.Location,
		//		heading: vl.Heading,
		DirTag: vl.DirTag,
		Point:  xf.ToPoint(vl.Location),
	}

	if _, ok := agency.Directions[report.DirTag]; !ok {
		if unknownDirTags == nil {
			unknownDirTags = make(map[string]bool)
		}
		if !unknownDirTags[report.DirTag] {
			log.Printf("Unknown direction: %v", report.DirTag)
			unknownDirTags[report.DirTag] = true
		}
	}

	d, err := xf.GeoHeadingToDirection(vl.Heading)
	if err != nil {
		report.Direction = geom.MakeDirectionFromVector(0, 0)
		debug_NoHeading++
		if debug_NoHeading < 20 {
			log.Printf("No heading report; d=%v, err=%v", d, err)
			log.Printf("      vl=%v", vl)
		}
	} else {
		report.Direction = geom.MakeDirection(d)
	}
	return
}

func LoadReports(
	filePath string, agency *nextbus.Agency, xf geogeom.CoordTransform) (
	reports []*Report, discardedRecords int, errors []error) {
	crc, err := util.OpenReadCsvFile(filePath)
	if err != nil {
		errors = append(errors, err)
		log.Printf("Error opening '%v': %v", filePath, err)
		return
	}
	defer crc.Close()
	fileRecords := 0
	for {
		record, err := crc.Read()
		if err != nil {
			if err != io.EOF {
				errors = append(errors, err)
				log.Printf("Error reading CSV file;\n\tFile: %s\n\tError: %v",
					filePath, err)
			}
			err = crc.Close()
			if err != nil {
				errors = append(errors, err)
				log.Print(err)
			}
			return
		}
		fileRecords++
		vl, err := nextbus.CSVFieldsToVehicleLocation(record)
		if err != nil {
			discardedRecords++
			errors = append(errors, err)
			log.Printf(
				"Error parsing field location record #%d;\n"+
					"\tRecord: %v\n\tFile: %s\n\tError: %v",
				fileRecords, record, filePath, err)
			// TODO Could add an error output channel/file to make it easier to
			// debug later.
			continue
		}
		report := NewReport(vl, agency, xf)
		if !report.DirectionIsValid() {
			discardedRecords++
			if discardedRecords < 20 {
				log.Printf("Location record has no heading; discarding: %v\n", vl)
			}
			continue
		}
		reports = append(reports, report)
	}

	log.Printf("Finished reading %d records from %s", fileRecords, filePath)
	return
}

func LinkReportsToSegments(
	allReports []*Report,
	segs []*PathSegment,
	maxDistance float64) {
	ClearNearReports(segs)
	ClearNearSegments(allReports)

	qt := CreateQuadTree(segs)
	for _, report := range allReports {
		segs := NearbyPathSegments(qt, report.Point, maxDistance)
		if len(segs) == 0 {
			//			discardedRecords++
			//		log.Printf("Found no subsegments near %v\n", report.Point)
			continue
		}
		for _, seg := range segs {
			LinkReportAndSegment(report, seg)
		}
		//		report.SortNearSegments()
	}
	return
}

func MedianWeightedReportDirection(
	report2weight map[*Report]float64) (dir geom.Direction, ok bool) {
	reports := make([]*Report, 0, len(report2weight))
	weights := make([]float64, 0, len(report2weight))
	for report, weight := range report2weight {
		if !report.DirectionIsValid() {
			continue
		}
		reports = append(reports, report)
		weights = append(weights, weight)
	}
	if len(reports) == 0 {
		dir = geom.InvalidDirection()
		return
	}
	ok = true
	lf := func() int { return len(reports) }
	xf := func(n int) float64 { return reports[n].UnitX() }
	yf := func(n int) float64 { return reports[n].UnitY() }
	wf := func(n int) float64 { return weights[n] }
	d2s := &stats.Data2DSourceDelegate{Lf: lf, Xf: xf, Yf: yf, Wf: wf}
	dir = geom.MakeDirection(stats.WeightedUnitVectorMedian(d2s))
	return
}

func MedianDistanceWeightedReportDirection(
	report2distance map[*Report]float64,
	computeWeight func(*Report, float64) float64) (dir geom.Direction, ok bool) {
	report2weight := make(map[*Report]float64)
	for report, distance := range report2distance {
		if !report.DirectionIsValid() {
			continue
		}
		w := computeWeight(report, distance)
		if w <= 0 {
			continue
		}
		report2weight[report] = w
	}
	return MedianWeightedReportDirection(report2weight)
}

func DetermineTrustworthyDirTags(
	reportsWithNearSegs []*Report,
	maxDistance float64) (dirTagsToTrust map[string]bool) {
	// Choose Route Directions that mostly appear to have reports going in
	// the same direction as the path.
	dirTag2Stats := MeasureRouteDirections(reportsWithNearSegs, maxDistance)
	dirTagsToTrust = make(map[string]bool)
	for dirTag, rs := range dirTag2Stats {
		if rs.Mean() < (math.Pi * 1 / 4) {
			dirTagsToTrust[dirTag] = true
		}
	}
	return
}
