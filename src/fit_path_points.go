package main
import (
	"flag"
	"log"
	"nextbus"
	"path/filepath"
	"util"
	"geom"
	"geo/geogeom"
	"geo"
	"math"
	"fmt"
//	"io"
	"fit"
	"nextbus/busgeom"
	"stats"
)
const (
	halfPi = math.Pi/2
	pi = math.Pi
	twoPi = math.Pi*2
)

func AngleWeight(angleBetween float64) float64 {
	// 0 (good) to 1 (bad)
	v := math.Min(halfPi, angleBetween) / halfPi
	return 1 - v
}

func DistanceWeight(distanceBetween float64) float64 {
	// 0 (good) to 1 (bad)
	if distanceBetween <= 0 { return 1 }
	return math.Min(1, 1 / distanceBetween)
}

func NearbyPathSegmentsAndDistances(
		qt geom.QuadTree, pt geom.Point,
		maxDistance float64) ([]*busgeom.PathSegment, []float64) {
	segs := busgeom.NearbyPathSegments(qt, pt, maxDistance)
	dists := make([]float64, len(segs))
	for i, seg := range segs {
		dists[i] = seg.DistanceToPoint(pt)
	}
	return segs, dists
}
func NearestPathSegmentInQuadTree(
		qt geom.QuadTree, pt geom.Point,
		maxDistance float64) (closest *busgeom.PathSegment, distance float64) {
	segs, dists := NearbyPathSegmentsAndDistances(qt, pt, maxDistance)
	distance = math.MaxFloat64
//	log.Printf("Found %d subsegments near point: %v", len(segs), pt)
	for i, seg := range segs {
		sd := dists[i]
//		log.Printf("Distance from %v to %v:  %.0f", pt, seg.seg, sd)
		if sd < distance {
			distance = sd
			closest = seg
		}
	}
	return
}
func NearestSubSegmentInSlice(
		pt geom.Point, segs []*busgeom.PathSegment) (
		closest *busgeom.PathSegment, distance float64) {
	distance = math.MaxFloat64
	for _, seg := range segs {
		sd := seg.DistanceToPoint(pt)
//		log.Printf("Distance from %v to %v:  %.0f", pt, seg.seg, sd)
		if sd < distance {
			distance = sd
			closest = seg
		}
	}
	return
}

// Return a value indicating how close the report location and direction are to
// the segment (i.e. on the segment at right angles isn't at all close, but a
// meter away pointed in the same direction is quite close).
func Weight(seg *busgeom.PathSegment, report *busgeom.Report) float64 {
//	distance := report.pt. seg.seg.
	return 1
}

// Returns a map from path to a list of paths that might follow that path.
func ConnectPaths(agency *nextbus.Agency) map[*nextbus.Path][]*nextbus.Path {
	log.Panicf("Not yet implemented")
	return nil
}

func PathLength(path []geom.Point) (length float64) {
	for ndx := 1; ndx < len(path); ndx++ {
		segLen := path[ndx-1].Distance(path[ndx])
		length += segLen

//		log.Printf("Seg %d: %v -> %v  (direction %3d, length %d)",
//							 ndx, path[ndx-1], path[ndx],
//							 int(geom.ToDegrees(path[ndx-1].DirectionTo(path[ndx])) + 0.5), int(segLen + 0.5))

//		log.Printf("Seg %d length: %.1f,   total: %.1f", ndx-1, segLen, length)
	}
	return
}

// Divide up a path which probably has lots of long segments and some
// short segments.  One purpose is to address oddities in the MBTA paths:
// at stops they take a 90 degree right turn from the center of the road
// the the curb, and then take a sharper turn (~135 degrees left back to
// the center of the road).  By just creating segments of a length
// considerably longer than these odd stop segments, we smooth out the path.
// Would probably be better if I generated variable length segments, shorter
// near sharper corners, longer on long straight segments.
func PartitionPath(path []geom.Point, targetSegLength float64) (
		result []geom.Point) {
	log.Printf("targetSegLength: %v", targetSegLength)
	totalLength := PathLength(path)
	log.Printf("totalLength: %v", totalLength)
	numSegs := math.Ceil(totalLength / targetSegLength)
	log.Printf("numSegs: %v", numSegs)
	if numSegs < 2 {
		return nil
	}
	
	adjustedTargetLength := totalLength / numSegs
	log.Printf("adjustedTargetLength: %v", adjustedTargetLength)

	result = make([]geom.Point, 0, int(numSegs) + 1)
	result = append(result, path[0])
	startIndex, startFraction := 0, 0.0
	for {
		endIndex, endFraction, end := FindNextEnd(
				path, adjustedTargetLength, startIndex, startFraction)
		result = append(result, end)
		if endIndex >= len(path) { break }
		startIndex, startFraction = endIndex, endFraction
	}
	return result
}	

func FindNextEnd(path []geom.Point, targetLength float64,
                 startIndex int, startFraction float64) (
		endIndex int, endFraction float64, end geom.Point) {
//	log.Printf("
	siLimit := len(path) - 1
	for startIndex < siLimit {
		fullSegLength := path[startIndex].Distance(path[startIndex+1])
		remainingSegLength := (1 - startFraction) * fullSegLength
		if remainingSegLength <= targetLength {
			targetLength -= remainingSegLength
			startIndex++
			startFraction = 0
			continue
		}
		// Ends here.
		remainingSegLength -= targetLength

		endIndex = startIndex
		endFraction = (fullSegLength - remainingSegLength) / fullSegLength

		delta := path[startIndex+1].Minus(path[startIndex])
		end.X = path[startIndex].X + delta.X * endFraction
		end.Y = path[startIndex].Y + delta.Y * endFraction
		return
	}
	return siLimit+1, 0, path[siLimit]
}

//// Originally broke into very short segments, but that seems too aggressive
//// for outlying paths, where some subsegments may have no reports for an entire month.
//func OLDPartitionNBPathToSubSegments(
//		transform geogeom.CoordTransform, nbpath []*nextbus.Location,
//		targetSegLength float64) (result []*SubSegment) {
//	points := geogeom.LocationsCollectionToPoints(
//			len(nbpath),
//			func(index int) geo.Location { return nbpath[index].Location },
//			transform)
//	newPoints := PartitionPath(points, targetSegLength)
//	if newPoints == nil {
//		log.Printf("Unable to partition path into smaller segments!")
//		newPoints = points
//	}
//	result = make([]*SubSegment, len(newPoints) - 1)
//	for ndx := 0; ndx < len(newPoints) - 1; ndx++ {
//		result[ndx] = NewSubSegment(newPoints[ndx], newPoints[ndx+1], ndx)
//	}
//	return
//}

func PartitionNBPathToSubSegments(
		transform geogeom.CoordTransform, nbpath []*nextbus.Location,
		targetSegLength float64) (result []*busgeom.PathSegment) {
	points := geogeom.LocationsCollectionToPoints(
			len(nbpath),
			func(index int) geo.Location { return nbpath[index].Location },
			transform)
	result = busgeom.MakePathSegments(points, nil)
	return
}

func FormatSeg(p *geom.DirectedSegment) string {
	return fmt.Sprintf("{%.1f, %.1f} => {%.1f, %.1f}   (direction %d, length %.1f)",
			p.Pt1.X, p.Pt1.Y, p.Pt2.X, p.Pt2.Y,
			int(geom.ToDegrees(p.Direction.Direction()) + 0.5),
			p.Length())
}

//func CosineSimilarity(a, b geom.Segment) {}

func FitLineForPathSegment(
		seg *busgeom.PathSegment, weightFunc busgeom.WeightFunction3) geom.Line {
	d2s := seg.MakeData2DSource(weightFunc)
	if d2s.Len() == 0 {
		log.Printf("TOO FEW REPORTS to compute a fit for segment: %#v", seg)
		return nil
	}

	line, err := fit.FitLineToPointsOR(d2s)
	if err != nil {
		log.Printf("Error fitting line for segment: %#v", seg)
		log.Printf("    Error: %v", err)
		return nil
	}

	if true {  // DEBUG OUTPUT
		pt1 := line.NearestPointTo(seg.Pt1)
		pt2 := line.NearestPointTo(seg.Pt2)
		newSeg := geom.NewDirectedSegment(pt1, pt2)
		log.Printf("OLD SEG: %v", FormatSeg(seg.DirectedSegment))
		log.Printf("NEW SEG: %v", FormatSeg(newSeg))

		numReports := d2s.Len()
		sumWeight := 0.0
		for ndx := 0; ndx < numReports; ndx++ {
			sumWeight += d2s.Weight(ndx)
		}

		log.Printf("Computed from %d points with an average weight of %.2f\n\n",
						 numReports, sumWeight / float64(numReports))
	}

	return line
}

func FitLineForSubSegment(
		seg *busgeom.PathSegment,
		maxBestDistance float64, maxDistance float64,
		useAngleWeight bool,
		report2distance map[*busgeom.Report]float64) geom.Line {
	// First pick points within maxDistance meters of the segment.
	reports := make([]*busgeom.Report, 0, len(report2distance))
	weights := make([]float64, 0, len(report2distance))
	sumWeight := 0.0
	for report, distance := range report2distance {
		if distance > maxDistance { continue }
		weight := 1.0
		if useAngleWeight {
			angle, ok := seg.AngleBetween(report)
			if !ok { continue }
			weight *= AngleWeight(angle)
		}
		if distance > maxBestDistance {
			weight *= DistanceWeight(distance)
		}
		reports = append(reports, report)
		weights = append(weights, weight)
		sumWeight += weight
	}
	if len(reports) == 0 {
		log.Printf("TOO FEW REPORTS to compute a fit for segment: %#v", seg)
		return nil
	}
	lf := func() int { return len(reports) }
	xf := func(n int) float64 { return reports[n].X }
	yf := func(n int) float64 { return reports[n].Y }
	wf := func(n int) float64 { return weights[n] }
	d2s := &stats.Data2DSourceDelegate{Lf:lf, Xf:xf, Yf:yf, Wf:wf}
	line, err := fit.FitLineToPointsOR(d2s)
	if err != nil {
		log.Printf("Error fitting line for segment: %#v", seg)
		log.Printf("    Error: %v", err)
		return nil
	}
	pt1 := line.NearestPointTo(seg.Pt1)
	pt2 := line.NearestPointTo(seg.Pt2)
	newSeg := geom.NewDirectedSegment(pt1, pt2)

	log.Printf("OLD SEG: %v", FormatSeg(seg.DirectedSegment))
	log.Printf("NEW SEG: %v", FormatSeg(newSeg))
	log.Printf("Computed from %d points with an average weight of %.2f\n\n",
						 len(reports), sumWeight / float64(len(reports)))
//	log.Print("\n\n")
	
	return geom.LineFromTwoPoints(pt1, pt2)
}

// Given a sequence of points, produces a segment between each pair, computes
// a linear fit for nearby points, and returns the linear fit corresponding
// to each segment.

func FitLinesForPath(
		pathPts []geom.Point, reports []*busgeom.Report,
		maxDistance float64) (result []geom.Line) {
	// Make segments.
	segs := busgeom.MakePathSegments(pathPts, nil)

	// Link segments and nearby points.
	busgeom.LinkReportsToSegments(reports, segs, maxDistance)

	// Which route directions can we trust to help us with determining the segment
	// direction (i.e. slope)?
	dirTagsToTrust := busgeom.DetermineTrustworthyDirTags(reports, maxDistance)

	// Fit a line for each segment of the path.
	lastNdx := len(segs) - 1
	for ndx, seg := range segs {
		// What is the likely slope of the line we'll fit to the points near seg?
		haveManyNearReports := (seg.NumNearReports() >= 50)
		weightFunc := func(seg *busgeom.PathSegment,
											 report *busgeom.Report,
											 rns *busgeom.ReportNearSegment) (weight float64) {
			if !dirTagsToTrust[report.DirTag] { return 0 }
			if rns.Distance > maxDistance { return 0 }

			weight = 1
			if !rns.IsPerpendicular {
				if haveManyNearReports {
					weight *= 0.5
				}
				if ndx == 0 || ndx == lastNdx {
					weight *= 0.5
				}
			}
			return weight
		}
		reportsDirection, ok := seg.MedianWeightedReportDirection(weightFunc)
		if !ok {
			reportsDirection = seg.Direction
		}

		// Define function for weighting points when fitting a line to the points.

		// TODO, maybe: For computing distance portion of weight, define a line that
		// goes through the midpoint of seg, with the slope from direction, and
		// compute distances between reports and that line.

		// If the median reports direction is close to that of the segment, then
		// we can use the IsPerpendicular value in rns.
		trustIsPerpendicular := reportsDirection.AngleBetween(seg.Direction) < (pi / 8)

		weightFunc = func(seg *busgeom.PathSegment,
											report *busgeom.Report,
											rns *busgeom.ReportNearSegment) (weight float64) {
			// Ignore points from before the beginning of, or after the end of,
			// the path.
			if trustIsPerpendicular {
				if ndx == 0 && rns.NearestPoint == seg.Pt1 {
					return 0
				}
				if ndx == lastNdx && rns.NearestPoint == seg.Pt2 {
					return 0
				}
			}

			weight = 1

			// Reports going the opposite direction are discarded, and those going
			// "adrift" are given lower weight.
			angleBetween := 0.0
			if report.DirectionIsValid() {
				angleBetween = reportsDirection.AngleBetween(report.Direction)
				if angleBetween >= halfPi { return 0 }
				weight *= (1 - (angleBetween / halfPi) * 0.3)
			}

			// Reports further away are given less weight.
			if rns.Distance > maxDistance { return 0 }
			weight *= (1 - (rns.Distance / maxDistance) * 0.3)

			if trustIsPerpendicular && !rns.IsPerpendicular {
				// Give less weight to those not perpendicular to the line segment
				// (i.e. we can't draw a line perpendicular to the line segment that
				// goes through the report point).
				weight *= 0.5
			}

			return weight
		}

		// Compute the line best fitting the points near the segment.
		
		line := FitLineForPathSegment(seg, weightFunc)
		if line == nil {
			log.Printf("Unable to fit line to path segment #%d", ndx)
			continue
		}

		result = append(result, line)
	}

	return
}

//func FitLinesForSubSegments(segs []*busgeom.PathSegment, maxDistance float64) {
//	maxCandidateDistance := maxDistance * 5
//	// Reports nearest to preceeding segments that might still be close enough.
//	candidateReports := make(map[*Report]float64)
//
//	for segIndex, ss := range segs {
//		// Eliminate candidates that are not close enough (on the assumption
//		// that the path doesn't have many hairpins).
//	
//	
//	
//	}
//
//
//
//}



var (
	// The flag package provides a default help printer via -h switch
	locationsGlobFlag = flag.String(
		"locations", "",
		"Path (glob) of locations csv file(s) to process")
	allPathsFlag = flag.String(
		"all-paths", "",
		"Path of xml file with description of all paths to be processed")
	pathIndexFlag = flag.Int(
		"path-index", -1,
		"Index of path to process")
	maxDistanceFlag = flag.Float64(
		"max-distance", 10,
		"Only points within this distance of the declared path will be used")
)

func main() {
	// Validate args.
	flag.Parse()
	ok := true

	// Are they set?
	if len(*locationsGlobFlag) == 0 {
		ok = false
		log.Print("--locations not set")
	}
	if len(*allPathsFlag) == 0 {
		ok = false
		log.Print("--all-paths not set")
	} else if !util.IsFile(*allPathsFlag) {
		ok = false
		log.Printf("Not a file: %v", *allPathsFlag)
	}
	if *pathIndexFlag <= 0 {
		ok = false
		log.Print("--path-index not set")
	}

	var agency *nextbus.Agency
	var path *nextbus.Path
	var matchingLocationFilePaths []string
	var err error
	if ok {
		// Read all-paths.xml to find path segments.
		log.Printf("Reading paths from: %s", *allPathsFlag)
		agency, err = nextbus.ReadPathsFromFile(*allPathsFlag)
		if err != nil {
			ok = false
			log.Println(err)
		} else if agency.NumPaths() == 0 {
			ok = false
			log.Printf("No paths in ", *allPathsFlag)
		}

		path = agency.GetPath(*pathIndexFlag)
		if path == nil {
			ok = false
			log.Printf("--path-index %d is not valid", *pathIndexFlag)
		}

		matchingLocationFilePaths, err = filepath.Glob(*locationsGlobFlag)
		if err != nil {
			ok = false
			log.Printf("Error processing --locations flag: %v", err)
		} else if len(matchingLocationFilePaths) == 0 {
			ok = false
			log.Print("--locations matched no files")
		}
	}

	if !ok {
		flag.PrintDefaults()
		return
	}

	log.Printf("Found path %d with %d waypoints", path.Index, len(path.WayPoints))

	// Create transform from lat-lon to meters, with the path waypoints in the
	// positive x, positive y quadrant (just to ease debugging, though it may
	// lower quality, compared to putting the center in the middle of the points).

	minLoc, maxLoc := path.Bounds()
	xf := geogeom.MakeMetricCoordTransform(minLoc)
	maxPt := xf.ToPoint(maxLoc)

	log.Printf("Lat-Lon Bounds %v X %v   (%.1f X %.1f meters)",
						 minLoc, maxLoc, maxPt.Y, maxPt.X)

	// Transform the path waypoints into points in the euclidean, metric space
	// centered on minLoc.
	points := geogeom.LocationsCollectionToPoints(
			len(path.WayPoints),
			func(index int) geo.Location { return path.WayPoints[index].Location },
			xf)
	origPoints := points

//
//	// Quadtree bounds
//	qb := geom.NewRectWithBorder(
//			0, math.Ceil(maxPt.X), 0, math.Ceil(maxPt.Y), 20, 20)
//
//	log.Printf("Quadtree bounds: %#v", qb)
//
//	// Create quadtree, for path region, insert metric path subsegments.
//	qt := geom.NewQuadTree(qb)
//	for i := range subSegments {
//		qt.Insert(subSegments[i])
//	}
//
//	log.Printf("Inserted %d segments into quadtree", len(subSegments))

	// Read specified vehicle location data, extracting just the location
	// and heading. Transform location and heading to metric, and find the
	// closest path subsegment(s) using the quadtree.

	totalDiscardedRecords := 0
	var allReports []*busgeom.Report

	for _, filePath := range matchingLocationFilePaths {
		log.Printf("Will read from '%v'", filePath)
		reports, numDiscardedReports, errors :=
				busgeom.LoadReports(filePath, agency, xf)

		if len(errors) > 0 {
			log.Printf("%d errors while reading file %v", len(errors), filePath)
		}

		allReports = append(allReports, reports...)
		totalDiscardedRecords += numDiscardedReports
	}

	log.Printf("Loaded %d total complete reports, and discarded %d records",
			len(allReports), totalDiscardedRecords)


	const maxRounds = 5
	for round := 1; round <= maxRounds; round++ {
		log.Println();
		log.Println("##############################################################");
		log.Println("Fitting round ", round);
		log.Println();

		lines := FitLinesForPath(points, allReports, *maxDistanceFlag)

		// TODO Error checking length of lines

		// TODO Merge lines that are nearly the same (perhaps apply regression to
		// determine if the lines are similar enough to merge).

		// TODO Ensure our lines are moving in the correct direction.

		firstPt := lines[0].NearestPointTo(points[0])
		newPoints := []geom.Point{firstPt}

		for ndx := 1; ndx < len(lines); ndx++ {
			pt, ok := lines[ndx-1].Intersection(lines[ndx])
			if !ok {
				log.Printf("Unable to intersect lines at segment %d", ndx)
				log.Printf("Line1: %v", lines[ndx-1])
				log.Printf("Line2: %v\n", lines[ndx])
				continue
			}
			newPoints = append(newPoints, pt)
		}
		lastPt := lines[len(lines)-1].NearestPointTo(points[len(points)-1])
		newPoints = append(newPoints, lastPt)

		points = newPoints
	}

	// TODO Compare paths

	if len(points) != len(origPoints) {
		log.Printf("Changed number of waypoints: %d  ->  %d", len(origPoints), len(points))
	}


	

//lastPt := points[len(points)-2]		
//			log.Printf("New seg: %v", FormatSeg(geom.NewDirectedSegment(lastPt, pt)))







//
//
//	// Choose Route Directions that mostly appear to have reports going in
//	// the same direction as the path.
//	dirTag2Stats := busgeom.MeasureRouteDirections(allReports, *maxDistanceFlag)
//	dirTagsToTrust := make(map[string]bool)
//	for dirTag, rs := range dirTag2Stats {
//		if rs.Mean() < (math.Pi * 1 / 4) {
//			dirTagsToTrust[dirTag] = true
//		}
//	}
//
//	for ndx, seg := range subSegments {
//		// Find reports near this segment that are trusted, and use them
//		// to determine a likely direction for the ground truth segment.
//		reports := busgeom.TrustedReportsNearSegment(seg, allReports, dirTagsToTrust)
//
//		
//
//		
//
//
//func MedianWeightedReportDirection(
//		report2weight map[*Report]float64) (dir geom.Direction, ok bool) {
//	
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
//
//
//


//	}
//
//	for i, seg := range subSegments {
//		r2d := seg2report2dist[seg]
//		numReps := len(r2d)
//		segLen := seg.Length()
//
//		log.Printf("SubSegment %d has %d nearby reports\n", i, numReps)
//		log.Printf("Length %.1f,   with %.2f reports per meter\n\n",
//				segLen, float64(numReps) / segLen)
//	}
//
//	log.Printf("\n")
//
//	busgeom.MeasureRouteDirections(qt, 7, allReports)

//	if true { return }
//
//	// For now, choosing to take the first and last points of the path as given,
//	// and only determine new points in-between.
//	points := []geom.Point{subSegments[0].Segment.Pt1,}
//	var lastLine geom.Line
//
//	for ndx, seg := range subSegments {
//		line := FitLineForSubSegment(seg, 5, 100, false, seg2report2dist[seg])
//		if line == nil {
//			log.Printf("Unable to fit line to path segment %d", ndx)
//			continue
//		}
//		if lastLine != nil {
//			pt, ok := lastLine.Intersection(line)
//			if !ok {
//				log.Printf("Unable to intersect lines at segment %d", ndx)
//				log.Printf("Line1: %v", lastLine)
//				log.Printf("Line2: %v\n", line)
//				continue
//			}
//			points = append(points, pt)
//			lastPt := points[len(points)-2]		
//			log.Printf("New seg: %v", FormatSeg(geom.NewDirectedSegment(lastPt, pt)))
//		}
//		lastLine = line
//	}

}
