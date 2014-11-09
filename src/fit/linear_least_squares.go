package fit

import (
	"fmt"
	"stats"
)

func FitLineToPoints(data stats.Data2DSource) (m, b float64, yIsDominant bool, err error) {
	n := float64(data.Len())

	if n < 2 {
		err = fmt.Errorf("not enough points: %d", n)
		return
	}

	var sum_x, sum_y, sum_xx, sum_xy, sum_yy float64
	for i, limit := 0, data.Len(); i < limit; i++ {
		x, y := data.X(i), data.Y(i)
		sum_x += x
		sum_y += y
		sum_xx += (x * x)
		sum_xy += (x * y)
		sum_yy += (y * y)
	}

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

	//	fmt.Printf("%v  %v %v %v %v\n     =>  %v %v   Y dominant\n\n",
	//						 n, sum_y, sum_x, sum_xy, sum_yy, m, b)
	return
}
