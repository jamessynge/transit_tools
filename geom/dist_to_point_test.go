package geom

import (
	"github.com/jamessynge/transit_tools/compare"
	"testing"
)

var (
	ptA = Point{0, 0}
	ptB = Point{1, 0}
	ptC = Point{0, 1}
	ptD = Point{1, 1}
	ptE = Point{0.70711, 0.70711}
	ptF = Point{-0.70711, 0.70711}
	ptM = Point{0.5, 0}
)

var testDistToPointData = []struct {
	name     string
	A, B, C  Point
	expected float64
}{
	{"A:B At A", ptA, ptB, ptA, 0.0},
	{"B:A At A", ptB, ptA, ptA, 0.0},

	{"A:B At B", ptA, ptB, ptB, 0.0},
	{"B:A At B", ptB, ptA, ptB, 0.0},

	{"A:B At M", ptA, ptB, ptM, 0.0},
	{"B:A At M", ptA, ptB, ptM, 0.0},

	{"A:B At C", ptA, ptB, ptC, 1.0},
	{"B:A At C", ptB, ptA, ptC, 1.0},

	{"A:B At D", ptA, ptB, ptD, 1.0},
	{"B:A At D", ptB, ptA, ptD, 1.0},

	{"A:B E", ptA, ptB, ptE, 0.70711},
	{"B:A E", ptB, ptA, ptE, 0.70711},

	{"A:M B", ptA, ptM, ptB, 0.5},
	{"B:M A", ptB, ptM, ptA, 0.5},

	{"F", ptA, ptB, ptF, 1.0},
}

func TestDistToPoint(t *testing.T) {
	for i := range testDistToPointData {
		e := &testDistToPointData[i]
		s := Segment{e.A, e.B}
		sqtSeg := newSQTSeg(s, nil)
		d := sqtSeg.DistToPoint(e.C)
		if !compare.NearlyEqual(e.expected, d) {
			t.Errorf("Case %v failed, computed %v", *e, d)
		}
	}
}
