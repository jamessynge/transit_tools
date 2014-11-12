package busgeom

import (
	"github.com/jamessynge/transit_tools/geom"
	"log"
	"math"
	"github.com/jamessynge/transit_tools/stats"
)

// Represents a segment of a path, from Pt1 to Pt2 (i.e. direction matters).
type PathSegment struct {
	*geom.DirectedSegment
	// Values used for identifying/locating the segment within the path.
	index           int
	offsetFromStart float64
	id              interface{}

	nearReports map[*Report]*ReportNearSegment
}

func NewPathSegment(pt1, pt2 geom.Point, index int,
	offsetFromStart float64, id interface{}) *PathSegment {
	p := &PathSegment{
		DirectedSegment: geom.NewDirectedSegment(pt1, pt2),
		index:           index,
		offsetFromStart: offsetFromStart,
		id:              id,
	}
	if id == nil {
		p.id = p
	}
	//	geom.InitDirectedSegment(&p.DirectedSegment, pt1, pt2)
	//	log.Printf("NewPathSegment: %#v", p)
	return p
}
func MakePathSegments(
	points []geom.Point, id interface{}) (result []*PathSegment) {
	result = make([]*PathSegment, len(points)-1)
	offsetFromStart := 0.0
	for ndx := 0; ndx < len(points)-1; ndx++ {
		result[ndx] = NewPathSegment(points[ndx], points[ndx+1], ndx, offsetFromStart, id)
		offsetFromStart += result[ndx].Length()
	}
	return
}

func (p *PathSegment) UniqueId() interface{} {
	return p.id
}
func (p *PathSegment) AngleBetween(report *Report) (angle float64, ok bool) {
	if !report.DirectionIsValid() {
		return
	}
	angle = p.Direction.AngleBetween(report.Direction)
	ok = true
	return
}
func (p *PathSegment) MedianWeightedReportDirection(
	weightFunc WeightFunction3) (dir geom.Direction, ok bool) {
	report2weight := make(map[*Report]float64)
	for report, rns := range p.nearReports {
		if !report.DirectionIsValid() {
			continue
		}
		w := weightFunc(p, report, rns)
		if w <= 0 {
			continue
		}
		report2weight[report] = w
	}
	return MedianWeightedReportDirection(report2weight)
}
func (p *PathSegment) NumNearReports() int {
	return len(p.nearReports)
}
func (p *PathSegment) MakeData2DSource(weightFunc WeightFunction3) stats.Data2DSource {
	var reports []*Report
	var weights []float64
	for report, rns := range p.nearReports {
		weight := weightFunc(p, report, rns)
		if weight > 0 {
			reports = append(reports, report)
			weights = append(weights, weight)
		}
	}
	lf := func() int { return len(reports) }
	xf := func(n int) float64 { return reports[n].X }
	yf := func(n int) float64 { return reports[n].Y }
	wf := func(n int) float64 { return weights[n] }
	return &stats.Data2DSourceDelegate{Lf: lf, Xf: xf, Yf: yf, Wf: wf}
}

func BoundsOfPathSegments(segs []*PathSegment) (result geom.Rect) {
	if len(segs) == 0 {
		return
	}
	result = segs[0].Bounds()
	for _, seg := range segs[1:] {
		b := seg.Bounds()
		result.MinX = math.Min(result.MinX, b.MinX)
		result.MaxX = math.Max(result.MaxX, b.MaxX)
		result.MinY = math.Min(result.MinY, b.MinY)
		result.MaxY = math.Max(result.MaxY, b.MaxY)
	}
	return
}

type VisitPathSegments struct {
	segs []*PathSegment
}

func (p *VisitPathSegments) Visit(datum geom.IntersectBounder) {
	seg, ok := datum.(*PathSegment)
	if !ok {
		log.Panicf("Wrong datum type: %T\nValue: %#v", datum, datum)
	}
	p.segs = append(p.segs, seg)
	//	log.Printf("%d: found segment %v in search region", len(p.segs), seg.id)
}
func NearbyPathSegments(
	qt geom.QuadTree, pt geom.Point,
	maxDistance float64) []*PathSegment {
	if qt == nil {
		return nil
	}
	rect := pt.ToRect(maxDistance, maxDistance)
	//	log.Printf("Searching for subsegments in: %v", rect)
	visitor := VisitPathSegments{}
	qt.Visit(rect, &visitor)
	//	log.Printf("Found %d subsegments near point: %v", len(visitor.segs), pt)
	return visitor.segs
}

func NearestPathSegmentInSlice(
	pt geom.Point, segs []*PathSegment) (
	closestSegment *PathSegment, closestPoint geom.Point,
	closestDistance float64) {
	closestDistance = math.MaxFloat64
	for _, seg := range segs {
		ptOnSeg, _ := seg.ClosestPointTo(pt)
		distance := pt.Distance(ptOnSeg)
		//		log.Printf("Distance from %v to %v:  %.0f", pt, seg.seg, sd)
		if distance < closestDistance {
			closestSegment = seg
			closestPoint = ptOnSeg
			closestDistance = distance
		}
	}
	return
}

func ClearNearReports(segs []*PathSegment) {
	for _, seg := range segs {
		seg.nearReports = make(map[*Report]*ReportNearSegment)
	}
}

func CreateQuadTree(segs []*PathSegment) (qt geom.QuadTree) {
	// Quadtree bounds
	bounds := BoundsOfPathSegments(segs)
	bounds = geom.NewRect(
		math.Floor(bounds.MinX), math.Ceil(bounds.MaxX),
		math.Floor(bounds.MinY), math.Ceil(bounds.MaxY))

	log.Printf("Quadtree bounds: %#v", bounds)

	// Create quadtree, for path region, and insert path subsegments.
	qt = geom.NewQuadTree(bounds)
	for _, seg := range segs {
		qt.Insert(seg)
	}
	return
}
