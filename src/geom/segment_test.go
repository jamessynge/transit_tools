package geom

import (
	"testing"
)

func Seg(x0, y0, x1, y1 float64) Segment {
	return Segment{Point{x0, y0}, Point{x1, y1}}
}
func TestClipping(t *testing.T) {
	testCases := []struct {
		name       string
		seg1, seg2 Segment
		intersects bool
	}{
		{"Vertical", Seg(-0.5, -10, -0.5, 10), Seg(-0.5, -1, -0.5, 1), true},
		{"Corners", Seg(-1, -1, 1, 1), Seg(-1, -1, 1, 1), true},
		{"Inside", Seg(0, 0, 0.1, 0.2), Seg(0, 0, 0.1, 0.2), true},
		{"VerticalOutside", Seg(-2, -10, -2, 10), Segment{}, false},
		{"VerticalOutside", Seg(1.0000001, -10, 1, 10), Segment{}, false},
		{"CornerOnly", Seg(11, -9, -9, 11), Seg(1, 1, 1, 1), true},
		{"Glancing", Seg(10.9, -9, -9.1, 11), Seg(1, 0.9, 0.9, 1), true},
	}
	r := NewRect(-1, 1, -1, 1)
	for i, tc := range testCases {
		clipped, intersects := tc.seg1.Clip(r)
		if intersects == tc.intersects && (!intersects || clipped.NearlyEqual(tc.seg2)) {
			continue
		}
		t.Errorf("testCases[%d] = %v\nclipped=%v\nintersects=%v",
			i, tc, clipped, intersects)
	}
}
