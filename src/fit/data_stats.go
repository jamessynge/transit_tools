package fit

import ()

////////////////////////////////////////////////////////////////////////////////
//type Data2DSource interface {

/*

func ComputeData2DStats(source Data2DSource) *Data2DStats {
	var sum_w, mean_x, mean_y, m2x, m2y float64
	n := source.Len()
	for i	:= 0 ; i < n; i++ {
		w := source.Weight(i)
		next_sum_w := w + sum_w

		x := source.X(i)
		delta_x := x - mean_x
		r_x = delta_x * w / next_sum_w
		mean_x += r_x
		m2x = m2x + sum_w * delta_x * r_x

		y := source.Y(i)
		delta_y := y - mean_y
		r_y = delta_y * w / next_sum_w
		mean_y += r_y
		m2y = m2y + sum_w * delta_y * r_y






		sum_w = next_sum_w
	}


def weighted_incremental_variance(dataWeightPairs):
    sumweight = 0
    mean = 0
    M2 = 0

    for x, weight in dataWeightPairs:  # Alternatively "for x, weight in zip(data, weights):"
        temp = weight + sumweight
        delta = x - mean
        R = delta * weight / temp
        mean = mean + R
        M2 = M2 + sumweight * delta * R  # Alternatively, "M2 = M2 + weight * delta * (x−mean)"
        sumweight = temp

    variance_n = M2/sumweight
    variance = variance_n * len(dataWeightPairs)/(len(dataWeightPairs) − 1)


}






func FitLineToPoints(points []geom.Point) (m, b float64, yIsDominant bool, err error) {
	n := float64(len(points))

	if n < 2 {
		err = fmt.Errorf("not enough points: %d", n)
		return
	}

	var sum_x, sum_y, sum_xx, sum_xy, sum_yy float64
	for i := range points {
		x, y := points[i].X, points[i].Y
		sum_x += x
		sum_y += y
		sum_xx += (x * x)
		sum_xy += (x * y)
		sum_yy += (y * y)
	}
*/
