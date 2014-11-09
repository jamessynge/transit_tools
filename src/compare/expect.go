package compare

import (
	"fmt"
	"math"
	"testing"
)

func ExpectEqual(logError func(args ...interface{}), expect, actual interface{}) {
	eq, differences := DeepCompare(expect, actual)
	if eq {
		return
	}
	msg := fmt.Sprintf("Not Equal!\nExpected: %v\nActual: %v\nDifferences:",
		expect, actual)
	for _, diff := range differences {
		if diff == nil {
			continue
		}
		msg = fmt.Sprintf("%s\nField: %v    Expected: %v     Actual: %v",
			msg, diff.FieldName, diff.Value1, diff.Value2)
	}
	logError(msg)
}

func ExpectTrue(t *testing.T, actual bool) {
	if actual {
		return
	}
	t.Fatal("Not True")
}

func EqualWithin(expected, actual, tolerance float64) bool {
	d := math.Abs(expected - actual)
	return d <= tolerance
}

func NearlyEqual(expected, actual float64) bool {
	tolerance := math.Max(math.Abs(expected/10000), 0.00001)
	return EqualWithin(expected, actual, tolerance)
}

func NearlyEqual3(expected, actual, tolerance float64) bool {
	return EqualWithin(expected, actual, tolerance)
}
