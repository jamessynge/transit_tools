package geom

import (
	"compare"
	"fmt"
	"strings"
	"testing"
)

type TestQTDatum struct {
	Segment
	id string
}

func NewTestQTDatum(pt1, pt2 Point, id string) *TestQTDatum {
	return &TestQTDatum{Segment{pt1, pt2}, id}
}

func (d TestQTDatum) UniqueId() interface{} {
	return d.id
}

// Fills with 10 segments along the x=y diagonal, each of length sqrt(2),
// and a single horizontal segment through x=5.5.
func fillTestQuadTree(tree QuadTree) {
	for i := 0; i < 10; i++ {
		pt1 := Point{float64(i), float64(i)}
		pt2 := Point{float64(i + 1), float64(i + 1)}
		datum := NewTestQTDatum(pt1, pt2, fmt.Sprint(i))
		tree.Insert(datum)
	}
	{
		pt1 := Point{-20, 5.5}
		pt2 := Point{20, 5.5}
		datum := NewTestQTDatum(pt1, pt2, "10")
		tree.Insert(datum)
	}
}

type IdSet map[string]bool

func (f IdSet) Visit(ib IntersectBounder) {
	datum := ib.(*TestQTDatum)
	f[datum.id] = true
}

func SplitStringToSet(in string) (out IdSet) {
	out = make(IdSet)

	for _, s := range strings.Split(in, ",") {
		if len(s) > 0 {
			out[s] = true
		}
	}
	return
}

func TestQuadTreeNearestSegment(t *testing.T) {
	tree := NewQuadTree(Rect{-100, 100, -100, 100})
	fillTestQuadTree(tree)

	testCases := []struct {
		name string
		r    Rect
		ids  string
	}{
		{"A", NewRectWithBorder(0, 0, 0, 0, 1, 1), "0"},
		{"A'", NewRect(0, 1, 0, 1), "0"},
		{"B", NewRectWithBorder(1, 1, 1, 1, 1, 1), "0,1"},
		{"C", NewRect(0, 1, 0, 10), "0,10"},
		{"D", NewRect(-1, 0, 0, 1), ""},
		{"E", NewRect(10, 11, 0, 10), "10"},
	}

	for _, tc := range testCases {
		f := make(IdSet)
		tree.Visit(tc.r, f)
		expect := SplitStringToSet(tc.ids)
		compare.ExpectEqual(t.Error, expect, f)
	}
}
