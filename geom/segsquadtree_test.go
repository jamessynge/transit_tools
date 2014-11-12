package geom

import (
	"github.com/jamessynge/transit_tools/compare"
	"testing"
)

func fillSQTree(tree *SQTree) {
	for i := 0; i < 10; i++ {
		pt1 := Point{float64(i), float64(i)}
		pt2 := Point{float64(i + 1), float64(i + 1)}
		seg := Segment{pt1, pt2}
		tree.InsertSegment(seg, i)
	}
	{
		pt1 := Point{-20, 5.5}
		pt2 := Point{20, 5.5}
		seg := Segment{pt1, pt2}
		tree.InsertSegment(seg, 10)
	}
}

var testNearestSegmentData = []struct {
	name      string
	x, y      float64
	threshold float64
	dist      float64
	data      int
}{
	{"A", 0, 0, 1, 0, 0},
	{"B", 1, 2, 1, 0.70711, 1},
	{"C", 1, 2, 0.5, 0, -1},
	{"D", 8, 9, 1, 0.70711, 8},
}

func TestNearestSegment(t *testing.T) {
	tree := NewSQTree(Rect{-100, 100, -100, 100})
	fillSQTree(tree)

	for i := range testNearestSegmentData {
		e := &testNearestSegmentData[i]
		p := Point{e.x, e.y}
		s, datax, dist := tree.NearestSegment(p, e.threshold)
		if datax == nil {
			if e.data < 0 {
				continue
			}
		} else if data, ok := datax.(int); ok {
			if compare.NearlyEqual(e.dist, dist) && e.data == data {
				continue
			}
		}
		t.Errorf("Case %v failed, computed %v, %v, %v", e.name, s, datax, dist)
	}
}
