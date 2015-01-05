package nbgeo

import (
	"strings"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geom"
	"github.com/jamessynge/transit_tools/nextbus"
)

type RouteToQuadTreeMap struct {
	agency *nextbus.Agency
	rt2qt  map[string]geom.QuadTree
	dt2qt  map[string]geom.QuadTree
}

func NewRouteToQuadTreeMap(agency *nextbus.Agency) *RouteToQuadTreeMap {
	p := &RouteToQuadTreeMap{
		agency: agency,
		rt2qt:  make(map[string]geom.QuadTree),
		dt2qt:  make(map[string]geom.QuadTree),
	}
	for _, route := range agency.Routes {
		qt := NewQuadTreeWithPaths(route.Paths)
		if qt == nil {
			glog.Infof("No paths for route '%s' (tag %s)", route.Title, route.Tag)
			continue
		}
		glog.Infof("Created quadtree for route %s with bounds: %v", route.Title, qt.Bounds())
		p.rt2qt[route.Tag] = qt
		for dirTag := range route.Directions {
			p.dt2qt[dirTag] = qt
		}
	}
	return p
}

// Given a vehicle location and tolerances (dx, dy),
func (p *RouteToQuadTreeMap) VLToPaths(
	vl *nextbus.VehicleLocation, dx, dy float64) []*nextbus.Path {
	rqt := p.rt2qt[vl.RouteTag]
	dqt := p.dt2qt[vl.DirTag]
	rPaths := NearbyPaths(rqt, vl.Location, dx, dy)
	if len(rPaths) == 0 {
		// Search a larger area if the original bounds were too tight.
		rPaths = NearbyPaths(rqt, vl.Location, dx*16, dy*16)
		glog.V(1).Infof("Expanded search yielded %d paths", len(rPaths))
	}
	if dqt != nil && rqt != dqt {
		if !strings.HasPrefix(vl.RouteTag, vl.DirTag) {
			glog.V(1).Infof("Dir tag (%s) isn't correct for route tag (%s)", vl.DirTag, vl.RouteTag)
		}
		// The dirTag doesn't identify a direction of the route identified
		// by routeTag). Also search for paths the route of dirTag.
		dPaths := NearbyPaths(dqt, vl.Location, dx, dy)
		if len(dPaths) == 0 {
			// Search a larger area if the original bounds were too tight.
			dPaths = NearbyPaths(dqt, vl.Location, dx*16, dy*16)
			glog.V(1).Infof("Expanded search yielded %d paths", len(dPaths))
		}
		if len(dPaths) > 0 {
			if len(rPaths) > 0 {
				// Add the dPaths entries into rPaths, if they're
				// not the same as those already in rPaths.
				origRPaths := rPaths
			OUTER:
				for _, dPath := range dPaths {
					for _, rPath := range origRPaths {
						if dPath == rPath {
							continue OUTER
						}
					}
					// New path.
					rPaths = append(rPaths, dPath)
				}
			} else {
				rPaths = dPaths
			}
		}
	}
	return rPaths
}
