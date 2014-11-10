package fit

import (
	"fmt"
	"geom"
	"math"
	"stats"
)

/*
Orthogonal regression, where the error terms for both x and y have the same
variance.  See:
	https://en.wikipedia.org/wiki/Orthogonal_regression
	https://en.wikipedia.org/wiki/Deming_regression

---- Ignoring sample bias ----

n = size of population

x_mean = population mean
       = sum(x)/n

y_mean = population mean
       = sum(y)/n

s_xx = population variance of x
     = sum(x_i^2)/n - x_mean^2

s_yy = population variance of x
     = sum(y_i^2)/n - y_mean^2

s_xy = population covariance
     = sum((x_i - x_mean)*(y_i - y_mean))/n
     = sum(x_i*y_i - x_i*y_mean - y_i*x_mean + x_mean*y_mean)/n
     = sum(x_i*y_i)/n - y_mean*sum(x_i)/n - x_mean*sum(y_i)/n + sum(x_mean*y_mean)/n
     = sum(x_i*y_i)/n - y_mean*x_mean - x_mean*y_mean + x_mean*y_mean
     = sum(x_i*y_i)/n - y_mean*x_mean

delta = variance(y_noise)/variance(x_noise)

m = slope of fit line

    s_yy - delta*s_xx + sqrt((s_yy - delta*s_xx)^2 + 4*delta*s_xy^2)
  = ----------------------------------------------------------------
               2*delta*s_xy

Since we assume that the bus location data has no particular bias to the
latitude and longitude (GPS) data, we further assume that delta = 1.

*/
func FitLineToPointsOR(data stats.Data2DSource) (line *geom.TwoPointLine, err error) {
	if data.Len() < 2 {
		err = fmt.Errorf("not enough points: %d", data.Len())
		return
	}

	statistics := stats.ComputeData2DStats(data)

	// Terms of quadratic formula, whose root leads us to the minimum of the sum
	// of squared residuals (i.e. best fit using orthogonal distance, squared,
	// from the data points to the line we determine).

	a := statistics.XYCovariance
	b := statistics.XVariance - statistics.YVariance
	c := statistics.XYCovariance

	numerator := -b + math.Sqrt(b*b+4*a*c)
	denominator := 2 * a

	if denominator != 0 {
		slope := numerator / denominator
		if -1 <= slope && slope <= 1 {
			// Slope within 45deg of horizontal. Compute points as a function of x.
			y_intercept := statistics.YMean - slope*statistics.XMean
			x1, x2 := statistics.XMean-statistics.XStdDev, statistics.XMean+statistics.XStdDev
			pt1 := geom.Point{x1, slope*x1 + y_intercept}
			pt2 := geom.Point{x2, slope*x2 + y_intercept}
			return geom.LineFromTwoPoints(pt1, pt2), nil
		}
	}

	if numerator != 0 {
		// Line is closer to vertical than to horizontal.  To keep accuracy from
		// dropping, compute the points as a function of y.
		slope := denominator / numerator // Inverted
		x_intercept := statistics.XMean - slope*statistics.YMean
		y1, y2 := statistics.YMean-statistics.YStdDev, statistics.YMean+statistics.YStdDev
		pt1 := geom.Point{slope*y1 + x_intercept, y1}
		pt2 := geom.Point{slope*y2 + x_intercept, y2}
		return geom.LineFromTwoPoints(pt1, pt2), nil
	}

	// There is apparently no correlation between changes in X and changes in Y
	// (i.e. XYCovariance == 0, so 2a and c are zero), and X has at least as
	// much variance as Y (i.e. XVariance-YVariance >= 0, so -b + sqrt(b^2) == 0).

	if statistics.XVariance == 0 {
		err = fmt.Errorf("invalid stats for producing a line: %#v", statistics)
		return
	}

	// We just have a horizontal line.

	slope := statistics.XYCovariance / statistics.XVariance
	y_intercept := statistics.YMean - slope*statistics.XMean
	x1, x2 := statistics.XMean-statistics.XStdDev, statistics.XMean+statistics.XStdDev
	pt1 := geom.Point{x1, slope*x1 + y_intercept}
	pt2 := geom.Point{x2, slope*x2 + y_intercept}
	return geom.LineFromTwoPoints(pt1, pt2), nil

	/*
		numerator := n*sum_xy - sum_x*sum_y
		denominator := n*sum_xx - sum_x*sum_x

		if denominator != 0 {
			m = numerator / denominator
			if -1 <= m && m <= 1 {
				// Slope within 45deg of horizontal
				b = (sum_y - m*sum_x) / n
				yIsDominant = false
				//			fmt.Printf("%v  %v %v %v %v\n     =>  %v %v  X dominant\n\n",
				//								 n, sum_x, sum_y, sum_xy, sum_xx, m, b)
				return
			}
		}

		// Slope is steep, where I found that the accuracy
		// drops, especially of the y intercept.  Therefore,
		// returning the line based on Y being the independent
		// variable (i.e run over rise, and X intercept).

		yIsDominant = true
		denominator = n*sum_yy - sum_y*sum_y
		if denominator == 0 {
			err = fmt.Errorf(
				"denominator is zero; n=%v sum_x=%v sum_y=%v sum_xx=%v sum_xy=%v sum_yy=%v",
				n, sum_x, sum_y, sum_xx, sum_xy, sum_yy)
		}

		m = numerator / denominator
		b = (sum_x - m*sum_y) / n

	*/

	err = fmt.Errorf("invalid stats for producing a line: %#v", statistics)
	return
}

/*


// pt1 and pt2 must be distinct else NearestPointTo will divide by zero.
func LineFromTwoPoints(pt1, pt2 Point) Line {


	// Note: this is the naive way to compute these values, which does not
	// account for arithmetic errors creeping in.  See these:
	//   https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance
	//   https://en.wikipedia.org/wiki/Compensated_summation
	//   https://en.wikipedia.org/wiki/Pairwise_summation
	// TODO Fix this, because we expect to have one of the bad cases: the
	// variance will be very small relative to the mean (i.e. even if we map the
	// Lat-Lon coords to meters, the mean will typically be somewhere in the range
	// [-10,000, 10,000], while the standard deviation will be in the range of a
	// few meters, so the variance will likely be in the range [5,100]).

	var sum_x, sum_y, sum_xx, sum_xy, sum_yy float64
	for i := range points {
		x, y := points[i].X, points[i].Y
		sum_x += x
		sum_y += y
		sum_xx += (x * x)
		sum_xy += (x * y)
		sum_yy += (y * y)
	}

	x_mean := sum_x / n
	y_mean := sum_y / n
	s_xx := sum_xx/n - x_mean*x_mean
	s_xy := sum_xy/n - x_mean*y_mean
	s_yy := sum_yy/n - y_mean*y_mean

	// Terms of quadratic formula, whose root leads us to the minimum of the sum
	// of squared residuals (i.e. best fit using orthogonal distance, squared,
	// from the data points to the line we determine).

	delta := 1.0  // Ratio of varians of the errors: var_y_error / var_x_error
							  // For now assume they have the same error, since we have no
							  // reason to assume otherwise.
	a := s_xy
	bb := delta * s_xx - s_yy
	c := s_xy

	numerator := -bb + math.Sqrt(bb*bb + 4*delta*a*c)
	denominator := 2 * delta * s_xy

	if denominator != 0 {
		m = numerator / denominator
		if -1 <= m && m <= 1 {
			// Slope within 45deg of horizontal
			b = (sum_y - m*sum_x) / n
			yIsDominant = false
			//			fmt.Printf("%v  %v %v %v %v\n     =>  %v %v  X dominant\n\n",
			//								 n, sum_x, sum_y, sum_xy, sum_xx, m, b)
			return
		}
	}

	// Slope is steep, where I found that the accuracy
	// drops, especially of the y intercept.  Therefore,
	// returning the line based on Y being the independent
	// variable (i.e run over rise, and X intercept).

	yIsDominant = true
	denominator = n*sum_yy - sum_y*sum_y
	if denominator == 0 {
		err = fmt.Errorf(
			"denominator is zero; n=%v sum_x=%v sum_y=%v sum_xx=%v sum_xy=%v sum_yy=%v",
			n, sum_x, sum_y, sum_xx, sum_xy, sum_yy)
	}

	m = numerator / denominator
	b = (sum_x - m*sum_y) / n

	//	fmt.Printf("%v  %v %v %v %v\n     =>  %v %v   Y dominant\n\n",
	//						 n, sum_y, sum_x, sum_xy, sum_yy, m, b)
	return
}
*/
