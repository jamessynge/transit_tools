package stats

import (
	"math"
)

type Data2DStats struct {
	// Population size
	N int

	// Population weight:
	//     sum_i(weight[i])
	// which is the number of data points when the average weight is 1, including
	// the unweighted cases.
	WSum float64
	//	WMean float64

	// Weighted Mean: sum_i(data[i]*weight[i])/WSum
	XMean, YMean float64

	// Weighted Variance: ( sum_i(data[i]^2*weight[i]) - weighted_mean^2 ) / WSum
	XVariance, YVariance float64

	// Weighted Co-variance:
	//   ( sum_i(x[i]*y[i]*weight[i]) - weighted_mean_x*weighted_mean_y ) / WSum
	XYCovariance float64

	// Standard deviation: sqrt(variance)
	XStdDev, YStdDev float64
}

func ComputeData2DStats(source Data2DSource) *Data2DStats {
	var sum_w, sum_wx, sum_wy, sum_wxx, sum_wxy, sum_wyy KahanSum

	n := source.Len()
	for i := 0; i < n; i++ {
		w := source.Weight(i)
		x := source.X(i)
		y := source.Y(i)

		sum_w.Add(w)
		sum_wx.Add(w * x)
		sum_wy.Add(w * y)
		sum_wxx.Add(w * x * x)
		sum_wxy.Add(w * x * y)
		sum_wyy.Add(w * y * y)
	}

	d := new(Data2DStats)
	d.N = n
	d.WSum = sum_w.Sum
	d.XMean = sum_wx.Sum / sum_w.Sum
	d.YMean = sum_wy.Sum / sum_w.Sum
	d.XVariance = sum_wxx.Sum/sum_w.Sum - d.XMean*d.XMean
	d.YVariance = sum_wyy.Sum/sum_w.Sum - d.YMean*d.YMean
	d.XYCovariance = sum_wxy.Sum/sum_w.Sum - d.XMean*d.YMean
	d.XStdDev = math.Sqrt(d.XVariance)
	d.YStdDev = math.Sqrt(d.YVariance)

	return d
}

// Returns the median X value and the median Y value (of weighted samples).
func Weighted2DVectorMedian(source Data2DSource) (xMedian, yMedian float64) {
	lf := func() int { return source.Len() }
	xf := func(n int) float64 { return source.X(n) }
	yf := func(n int) float64 { return source.Y(n) }
	wf := func(n int) float64 { return source.Weight(n) }

	xMedian, _ = Data1DWeightedMedian(&Data1DSourceDelegate{L: lf, V: xf, W: wf})
	yMedian, _ = Data1DWeightedMedian(&Data1DSourceDelegate{L: lf, V: yf, W: wf})
	return
}

// X and Y in source represent unit vectors (points on the circle). Returns
// the weighted median angle.
func WeightedUnitVectorMedian(source Data2DSource) float64 {
	xMedian, yMedian := Weighted2DVectorMedian(source)
	return math.Atan2(yMedian, xMedian)
}
