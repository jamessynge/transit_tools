package geo

import (
	"fmt"
	"math"

	"github.com/golang/glog"
)

const EarthRadiusAtEquator Meters = 6378137
const EarthRadiusAtPoles Meters = EarthRadiusAtEquator - Meters(21384.7)

// Format as degrees, minutes and seconds, with a suffix for sign.
func formatDMS(v float64, neg, pos string) string {
	var suffix string
	if v < 0 {
		suffix = neg
		v = -v
	} else {
		suffix = pos
	}
	deg := int(v)
	v -= float64(deg)
	v *= 60
	min := int(v)
	v -= float64(min)
	v *= 60
	return fmt.Sprintf("%d.%02d.%04.1f%s", deg, min, v, suffix)
}

func toRadians(degrees float64) float64 {
	return degrees * math.Pi / 180.0
}
func toDegrees(radians float64) float64 {
	return radians * 180.0 / math.Pi
}

func MeasureCentralAxes(
	loLon, hiLon Longitude, loLat, hiLat Latitude) (
	snDistance, weDistance float64) {

	var south, north, west, east Location
	south.Lat = loLat
	north.Lat = hiLat
	west.Lon = loLon
	east.Lon = hiLon

	south.Lon = (loLon + hiLon) / 2
	north.Lon = south.Lon
	west.Lat = (loLat + hiLat) / 2
	east.Lat = west.Lat

	// Measure the distance.
	snDistance, _ = ToDistanceAndHeading(south, north)
	weDistance, _ = ToDistanceAndHeading(west, east)

	if glog.V(1) {
		glog.Infof("MeasureCentralAxes: s2n: %s to %s  =>   %.3f Km",
							 loLat.FormatDMS(), hiLat.FormatDMS(), snDistance / 1000)
		glog.Infof("MeasureCentralAxes: w2e: %s to %s  =>   %.3f Km",
							 loLon.FormatDMS(), hiLon.FormatDMS(), weDistance / 1000)
	}

	return
}

func computeAreaBisectorCompare3(
		loLon, hiLon Longitude, loLat, midLat, hiLat Latitude,
		height float64,
		loWidth, midWidth, hiWidth float64) float64 {
	// Are the 3 west-east distances very similar?
	r1 := math.Abs(midWidth / loWidth - 1)
	r2 := math.Abs(midWidth / hiWidth - 1)
	r3 := math.Abs(hiWidth / loWidth - 1)

	if glog.V(1) {
		glog.Infof("computeAreaBisectorCompare3: widths, Km:  %.3f,  %.3f,  %.3f",
							 loWidth / 1000, midWidth / 1000, hiWidth)
		glog.Infof("ratios - 1: %.3f %.3f %.3f", r1, r2, r3)
	}

	halfHeight := height / 2.0
	if r1 <= 0.02 || r2 <= 0.02 || r3 <= 0.02 {
		// Within 2% of each other, good enough for now.
		return ((halfHeight  * (loWidth + midWidth) / 2) +
					  (halfHeight * (midWidth + hiWidth) / 2))
	}

	// Sub-divide further.

	loArea := computeAreaBisector(loLon, hiLon, loLat, midLat, halfHeight, loWidth, midWidth)
	hiArea := computeAreaBisector(loLon, hiLon, midLat, hiLat, halfHeight, midWidth, hiWidth)
	return loArea + hiArea
}

func computeAreaBisector(
		loLon, hiLon Longitude, loLat, hiLat Latitude,
		height float64,
		loWidth, hiWidth float64) float64 {

	// Need to compute the latitude midway between loLat and hiLat, then
	// the width along that latitude between loLon and hiLon.
	midLat := (loLat + hiLat) / 2

	var west, east Location
	west.Lon = loLon
	west.Lat = midLat
	east.Lon = hiLon
	east.Lat = midLat
	midWidth, _ := ToDistanceAndHeading(west, east)

	return computeAreaBisectorCompare3(
			loLon, hiLon, loLat, midLat, hiLat, height, loWidth, midWidth, hiWidth)
}


func MeasureCentralAxesAndArea(
	loLon, hiLon Longitude, loLat, hiLat Latitude) (
	snDistanceMeters, weDistanceMeters, areaSquareMeters float64) {
	// Note that this function could be greatly simplified if we choose to
	// assume that the earth is a sphere, not an oblate spheroid (i.e. that
	// the radius is constant).

	// Measure the south to north distance (same on all lines of longitude).
	var south, north Location
	south.Lat = loLat
	north.Lat = hiLat
	south.Lon = loLon
	north.Lon = south.Lon
	snDistanceMeters, _ = ToDistanceAndHeading(south, north)

	// Measure the width at the bottom and top of the lat-lon rectangle.
	var west, east Location
	west.Lon = loLon
	east.Lon = hiLon
	west.Lat = loLat
	east.Lat = west.Lat
	loWidth, _ := ToDistanceAndHeading(west, east)

	west.Lat = hiLat
	east.Lat = west.Lat
	hiWidth, _ := ToDistanceAndHeading(west, east)

	areaSquareMeters = computeAreaBisector(
			loLon, hiLon, loLat, hiLat, snDistanceMeters, loWidth, hiWidth)
	weDistanceMeters = areaSquareMeters / snDistanceMeters

	if glog.V(1) {
		glog.Infof("MeasureCentralAxesAndArea: s2n: %s to %s  =>   %.3f Km",
							 loLat.FormatDMS(), hiLat.FormatDMS(), snDistanceMeters / 1000)
		glog.Infof("MeasureCentralAxesAndArea: w2e: %s to %s  =>   %.3f Km",
							 loLon.FormatDMS(), hiLon.FormatDMS(), weDistanceMeters / 1000)
		glog.Infof("MeasureCentralAxesAndArea: area = %.3f Km²", areaSquareMeters / 1000000)
	}
	return
}
