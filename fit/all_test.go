package fit

import (
	"github.com/jamessynge/transit_tools/compare"
	_ "fmt"
	"github.com/jamessynge/transit_tools/geom"
	"math"
	"math/rand"
	_ "sort"
	"github.com/jamessynge/transit_tools/stats"
	"testing"
)

var (
	rngSource = rand.NewSource(12345)
	rng       = rand.New(rngSource)
)

func CreatePoints(yIsDominant bool, m, b float64, n int) []geom.Point {
	points := make([]geom.Point, n)
	for i := range points {
		u := (rng.Float64() - 0.5) * 100 // [-50,50)
		v := m*u + b
		if yIsDominant {
			u, v = v, u
		}
		points[i].X = u
		points[i].Y = v
	}
	return points
}

//type errorFunc func() (ue, ve float64)

func CreateOrthoPoints(
	yIsDominant bool, m, b float64, n int, errorStdDev float64) []geom.Point {
	points := make([]geom.Point, n)

	// Want to move point u,v by a distance e in the direction perpendicular
	// to m, so that the "errors" are orthogonal to the line.
	var errors func(e float64) (float64, float64)
	if m != 0 {
		perp := -1 / m // Slope of line perpendicular to line with slope m
		errors = func(e float64) (ue, ve float64) {
			// Want to move point u,v by a distance e in the direction perpendicular
			// to m.  Move u by ue, and v by ve.  Determine ue, ve from perp and e.
			//		e^2 = ue^2 + ve^2                 // Pythagorean Theorem
			//		e^2 = ue^2 + (perp*ue)^2          // ve is a function of perp and ue
			//		e^2 = ue^2*(1 + perp^2)           // Factor out ue^2
			//		e^2 / (1 + perp^2) = ue^2         // ue as a function of perp and e
			ue = math.Sqrt(e * e / (1 + perp*perp))
			if e < 0 {
				ue = -ue
			}
			ve = perp*ue + 0
			return
		}
	} else {
		// The line is horizontal, so the errors are all in the vertical direction
		// (i.e. ue is always zero, and ve is just the distance e used above.
		errors = func(e float64) (ue, ve float64) {
			ue, ve = 0, e
			return
		}
	}

	for i := range points {
		u := (rng.Float64() - 0.5) * 100 // [-50,50)
		v := m*u + b
		ue, ve := errors(rng.NormFloat64() * errorStdDev)
		u += ue
		v += ve
		if yIsDominant {
			u, v = v, u
		}
		points[i].X = u
		points[i].Y = v
	}
	return points
}
func JostlePoints(by float64, points []geom.Point) {
	if by <= 0 {
		return
	}
	for i := range points {
		points[i].X += math.Max(-by*3, math.Min(rng.NormFloat64()*by, by*3))
		points[i].Y += math.Max(-by*3, math.Min(rng.NormFloat64()*by, by*3))
	}
}
func SwapXY(points []geom.Point) {
	for i := range points {
		points[i].X, points[i].Y = points[i].Y, points[i].X
	}
}
func TranslatePoints(dx, dy float64, points []geom.Point) {
	for i := range points {
		points[i].X += dx
		points[i].Y += dy
	}
}

type PointSlice []geom.Point

func (d PointSlice) Len() int { return len(d) }
func (d PointSlice) Less(i, j int) bool {
	return d[i].X < d[j].X
}
func (d PointSlice) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func TestFitLineToPointsOR(t *testing.T) {
	errorStdDev := 2.0
	var angleDeltaStats, allDistanceStats stats.Running1DStats
	for angle := 0; angle < 360; angle++ {
		var m, b float64
		var yIsDominant bool
		var points []geom.Point

		if (45 < angle && angle < 135) ||
			(225 < angle && angle < 315) {
			yIsDominant = true
			yxAngle := 90 - angle
			m = math.Tan(float64(yxAngle) / 180 * math.Pi)
			b = 0.0 //rng.Float64() * 10
		} else {
			yIsDominant = false
			m = math.Tan(float64(angle) / 180 * math.Pi)
			b = 0.0 //rng.Float64() * 10
		}
		//		t.Logf("angle %d  =>  %v, %v, %v", angle, yIsDominant, m, b)

		points = CreateOrthoPoints(yIsDominant, m, b, 1000, errorStdDev)

		// Move away from the origin so we can see the effect of doing so.
		dx, dy := (rng.Float64()*2-1)*20000, (rng.Float64()*2-1)*20000
		TranslatePoints(dx, dy, points)

		line2pt, e2 := FitLineToPointsOR(geom.PointSlice(points))

		if e2 != nil || line2pt == nil {
			t.Errorf("\ntest case: %v, %v, %v, %v\nerror: %v\n",
				angle, m, b, yIsDominant, e2)
			continue
		}

		expected := float64(angle % 180)
		actual := line2pt.Angle() * 180.0 / math.Pi
		delta := expected - actual //math.Abs(expected - actual)
		if delta > 90 {
			delta -= 180
		} else if delta < -90 {
			delta += 180
		}

		angleDeltaStats.Add(delta)

		//		t.Logf("angle: %3d   actual: %6.2f   delta: %7.4f", angle, actual, delta)

		if math.Abs(delta) > 0.5 {
			if true {
				t.Errorf("angle: %3d   actual: %6.2f   delta: %7.4f", angle, actual, delta)
			} else {
				t.Errorf("\ntest case: %v, %v, %v, %v\nWrong angle: %v    %#v\n",
					angle, m, b, yIsDominant, line2pt.Angle()*180.0/math.Pi, line2pt)
			}
			continue
		}

		// Check if points are near expected position.
		var distanceStats stats.Running1DStats
		for i := range points {
			linePt := line2pt.NearestPointTo(points[i])
			distance := linePt.Distance(points[i])
			if i%2 == 0 {
				distance = -distance
			}
			distanceStats.Add(distance)
			allDistanceStats.Add(distance)
		}

		t.Logf("Angle %d distance stats: %v", angle, &distanceStats)

		tolerance := 0.1 * errorStdDev
		if !compare.NearlyEqual3(0, distanceStats.Mean(), tolerance) ||
			!compare.NearlyEqual3(errorStdDev, distanceStats.StandardDeviation(), tolerance) {
			t.Errorf("Angle %d distances not as expected", angle)
		}
	}

	t.Logf("Angle delta stats: %v", &angleDeltaStats)

	if math.Abs(angleDeltaStats.Mean()) > 0.01 ||
		angleDeltaStats.StandardDeviation() > 0.2 {
		t.Error("Angles too far from expected")
	}

	t.Logf("All distance stats: %v", &allDistanceStats)

	if !compare.NearlyEqual3(0, allDistanceStats.Mean(), 0.001) ||
		!compare.NearlyEqual3(errorStdDev, allDistanceStats.StandardDeviation(), 0.001) {
		t.Error("All distances not as expected")
	}
}

/*
func TestFitLineToPoints(t *testing.T) {
	testCases := []struct {
		m, b      float64 // slope and intercept of data
		n         int     // num points to create
		jostle    float64 // amount of random adjustment to points
		tolerance float64
	}{
		{0, 0, 100, 0.01, 0.01},
		{0.5, 1, 100, 0.001, 0.01},
		{0.9, 1, 10, 0.001, 0.01},
	}
	for _, tc := range testCases {
		//		t.Log("create points...")
		points := CreatePoints(false, tc.m, tc.b, tc.n)
		//		t.Log("jostle points...")
		JostlePoints(tc.jostle, points)
		//		var ps PointSlice = points
		//		sort.Sort(ps)
		//		t.Log(ps)
		m, b, yDom, _ := FitLineToPoints(points)
		if compare.EqualWithin(tc.m, m, tc.tolerance) &&
			compare.EqualWithin(tc.b, b, tc.tolerance) {
			continue
		}
		t.Errorf("\ntest case: %v\nresult: %v, %v, %v\n", tc, m, b, yDom)
	}
}
func Test2FitLineToPoints(t *testing.T) {
	jostleBy := 0.1
	for angle := 0; angle < 360; angle++ {
		var m, b float64
		var yIsDominant bool
		var points []geom.Point

		if (45 < angle && angle < 135) ||
			(225 < angle && angle < 315) {
			yIsDominant = true
			yxAngle := 90 - angle
			m = math.Tan(float64(yxAngle) / 180 * math.Pi)
			b = 1.0 //rng.Float64() * 10
		} else {
			yIsDominant = false
			m = math.Tan(float64(angle) / 180 * math.Pi)
			b = 1.0 //rng.Float64() * 10
		}
		t.Logf("angle %d  =>  %v, %v, %v", angle, yIsDominant, m, b)

		//		t.Log("create points...")
		points = CreatePoints(yIsDominant, m, b, 100)
		//		t.Log("jostle points...")
		JostlePoints(jostleBy, points)

		//		var ps PointSlice = points
		//		sort.Sort(ps)
		//		t.Log(ps)

		m2, b2, y2, e2 := FitLineToPoints(points)
		//		if compare.EqualWithin(m, m2, 0.01) &&
		//		   compare.EqualWithin(b, b2, 0.01) &&
		//		   yIsDominant == y2 &&
		//		   e2 == nil {
		//		  continue
		//		}
		if e2 != nil {
			t.Errorf("\ntest case: %v, %v, %v,  %v\nerror: %v\n",
				angle, m, b, yIsDominant, e2)
			continue
		}
		// Check if points are near expected position.
		distanceLimit := math.Hypot(jostleBy, jostleBy) * 1.5
		for i := range points {
			var pt geom.Point
			if y2 {
				pt.Y = points[i].Y
				pt.X = m2*pt.Y + b2
			} else {
				pt.X = points[i].X
				pt.Y = m2*pt.X + b2
			}
			distance := pt.Distance(points[i])
			if distance > distanceLimit {
				t.Errorf("point too far: %v > %v\n    test point: %v\ncomputed point: %v",
					distance, distanceLimit, points[i], pt)
			}
		}
	}
}
*/
