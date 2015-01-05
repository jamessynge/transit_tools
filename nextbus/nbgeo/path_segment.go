package nbgeo

import (
	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/geom"
	"github.com/jamessynge/transit_tools/nextbus"
)

func LLToPoint(latlon *geo.Location) geom.Point {
	return geom.Point{X: float64(latlon.Lon), Y: float64(latlon.Lat)}
}

func AddPathToQuadTreeAsSegs(path *nextbus.Path, qt geom.QuadTree) {
	//	id := fmt.Sprint(path.Index)
	for index, limit := 0, len(path.WayPoints)-1; index < limit; index++ {
		wp1, wp2 := path.WayPoints[index], path.WayPoints[index+1]
		ps := &PathLLSeg{
			path:  path,
			index: index,
			//			id:    id,
			seg: geom.Segment{
				Pt1: LLToPoint(&wp1.Location),
				Pt2: LLToPoint(&wp2.Location),
			},
		}
		qt.Insert(ps)
	}
}

func NewQuadTreeWithAgencyPaths(agency *nextbus.Agency) geom.QuadTree {
	min, max, ok := agency.GetPathBounds()
	if !ok {
		glog.Fatal("Failed to find bounds for paths!")
	}
	qt := geom.NewQuadTree(
		geom.NewRect(
			float64(min.Lon), float64(max.Lon),
			float64(min.Lat), float64(max.Lat)))
	glog.Infof("Created quadtree with bounds: %v", qt.Bounds())
	for _, path := range agency.PathsByIndex {
		AddPathToQuadTreeAsSegs(path, qt)
	}
	return qt
}

func NewQuadTreeWithPaths(paths []*nextbus.Path) geom.QuadTree {
	min, max, ok := nextbus.BoundsOfPaths(paths)
	if !ok {
		return nil
	}
	qt := geom.NewQuadTree(
		geom.NewRect(
			float64(min.Lon), float64(max.Lon),
			float64(min.Lat), float64(max.Lat)))
	for _, path := range paths {
		AddPathToQuadTreeAsSegs(path, qt)
	}
	return qt
}

type PathLLSeg struct {
	path  *nextbus.Path
	index int
	//	id    string
	seg geom.Segment
}

func (p *PathLLSeg) UniqueId() interface{} {
	return p.path
}

func (p *PathLLSeg) IntersectBounds(r geom.Rect) (intersection geom.Rect, empty bool) {
	return p.seg.IntersectBounds(r)
}

func (p *PathLLSeg) Intersects(r geom.Rect) bool {
	return p.seg.Intersects(r)
}

type visitPathLLSeg struct {
	//	latlon geo.Location
	//	point geom.Point
	paths []*nextbus.Path
	v1    glog.Verbose
}

/* TODO Update for new QuadTreeVisitor interface definition

func (p *visitPathLLSeg) Visit(datum geom.QuadTreeDatum) {
	ps, ok := datum.(*PathLLSeg)
	if !ok {
		log.Panicf("Wrong datum type: %T\nValue: %#v", datum, datum)
	}
	p.paths = append(p.paths, ps.path)
	//	log.Printf("%d: found path index %d in search region",
	//	           len(p.paths), ps.path.Index)
}

func (p *visitPathLLSeg) Visit(datum IntersectBounder) {
	ps, ok := datum.(*PathLLSeg)
	if !ok {
		log.Panicf("Wrong datum type: %T\nValue: %#v", datum, datum)
	}
	p.paths = append(p.paths, ps.path)
	//	log.Printf("%d: found path index %d in search region",
	//	           len(p.paths), ps.path.Index)
}
*/
func (p *visitPathLLSeg) Visit(datum geom.IntersectBounder) {
	ps, ok := datum.(*PathLLSeg)
	if !ok {
		glog.Fatalf("Wrong datum type: %T\nValue: %#v", datum, datum)
	}
	p.paths = append(p.paths, ps.path)
	if p.v1 {
		glog.Infof("%d: found path index %d in search region", len(p.paths), ps.path.Index)
	}
}

func NearbyPaths(qt geom.QuadTree, loc geo.Location, dx, dy float64) []*nextbus.Path {
	if qt == nil {
		return nil
	}
	v1 := glog.V(1)
	point := LLToPoint(&loc)
	rect := point.ToRect(dx, dy)
	v1.Infof("Searching for paths in: %v", rect)
	visitor := visitPathLLSeg{v1: v1}
	qt.Visit(rect, &visitor)
	v1.Infof("Found %d paths near location: %v", len(visitor.paths), loc)
	return visitor.paths
}
