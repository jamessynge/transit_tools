package fit

import (
	"fmt"
	"github.com/jamessynge/transit_tools/geom"
	"github.com/jamessynge/transit_tools/stats"
	"math"
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

func OrthoRegrFitLineToStats(statistics *stats.Data2DStats) (line *geom.TwoPointLine, err error) {
	if statistics.N < 2 {
		err = fmt.Errorf("not enough points: %d", statistics.N)
		return
	}

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
}

func FitLineToPointsOR(data stats.Data2DSource) (line *geom.TwoPointLine, err error) {
	if data.Len() < 2 {
		err = fmt.Errorf("not enough points: %d", data.Len())
		return
	}

	return OrthoRegrFitLineToStats(stats.ComputeData2DStats(data))
}
