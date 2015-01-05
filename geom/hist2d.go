package geom

// 2-dimensional histogram, which is really just a 2-d matrix,
// mapped onto some other (axis aligned) coordinate system referred to as
// the data coordinates.
// We count up the number of data points that correspond to each bucket as
// the intensity at that bucket.
// We can then count up the number of times each intensity occurs in the
// matrix, producing a 1-d histogram of intensities (i.e. a map from intensity
// to count of occurences).

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/golang/glog"
)

type HistCount uint32
type HistCountOccurrences map[HistCount]HistCount
type IntensityCount struct {
	Intensity, Count HistCount
}
type IntensityCountSlice []IntensityCount

type Hist2D struct {
	// Data coordinates region (i.e. data outside here is ignored).
	DataBounds            Rect
	dataWidth, dataHeight float64
	// Number of buckets in the histogram x-axis
	BucketWidth int
	// Number of buckets in the histogram y-axis
	BucketHeight int
	Data         [][]HistCount
	MaxVal       HistCount
	NumNonZero   HistCount
}

func NewHist2D(dataBounds Rect, bucketWidth, bucketHeight int) *Hist2D {
	p := &Hist2D{
		DataBounds:   dataBounds,
		dataWidth:    dataBounds.Width(),
		dataHeight:   dataBounds.Height(),
		BucketWidth:  bucketWidth,
		BucketHeight: bucketHeight,
		Data:         make([][]HistCount, bucketHeight),
	}
	for rowIndex := range p.Data {
		p.Data[rowIndex] = make([]HistCount, bucketWidth)
	}
	return p
}
func (p *Hist2D) YToRow(y float64) (row int) {
	relative := float64(y) - p.DataBounds.MinY
	frac := relative / p.dataHeight
	row = int(frac * float64(p.BucketHeight))
	return
}
func (p *Hist2D) XToColumn(x float64) (column int) {
	relative := float64(x) - p.DataBounds.MinX
	frac := relative / p.dataWidth
	column = int(frac * float64(p.BucketWidth))
	return
}

// Note this swaps horizontal and vertical axes.
func (p *Hist2D) DataPtToRC(pt Point) (row, column int) {
	return p.YToRow(pt.Y), p.XToColumn(pt.X)
}
func (p *Hist2D) RCToImageXY(row, column int) (x, y int) {
	return column, p.BucketHeight - row
}
func (p *Hist2D) DataPtToImageXY(pt Point) (x, y int) {
	row, column := p.DataPtToRC(pt)
	return p.RCToImageXY(row, column)
}
func (p *Hist2D) DataXYToImageXY(dataX, dataY float64) (x, y int) {
	column := p.XToColumn(dataX)
	row := p.YToRow(dataY)
	return p.RCToImageXY(row, column)
}

func (p *Hist2D) IncrementRC(row, column int) {
	if 0 <= row && row < p.BucketHeight && 0 <= column && column < p.BucketWidth {
		rowSlice := p.Data[row]
		val := rowSlice[column] + 1
		rowSlice[column] = val
		if val == 1 {
			p.NumNonZero++
		}
		if p.MaxVal < val {
			p.MaxVal = val
		}
	} else {
		glog.Infof("rc out of range: %d, %d", row, column)
	}
}
func (p *Hist2D) IncrementPt(pt Point) {
	row, column := p.DataPtToRC(pt)
	p.IncrementRC(row, column)
}
func (p *Hist2D) TotalSlots() int {
	return p.BucketWidth * p.BucketHeight
}
func (p *Hist2D) HistCountOccurrences() (countsMap HistCountOccurrences) {
	countsMap = make(HistCountOccurrences)
	for _, rowSlice := range p.Data {
		for _, count := range rowSlice {
			countsMap[count]++
		}
	}
	return
}

// Produce the histogram of intensity values in this Hist2D (i.e. how many
// times each count occurs).
func (p *Hist2D) CountOccurrencesAsSlice() IntensityCountSlice {
	countsMap := p.HistCountOccurrences()
	return countsMap.ToSlice()
}

// Returns the 1-d histogram (slice of intensity and occurrences of that
// intensitie, and ) of
func (p *Hist2D) CountsAndCDF() (
	counts, cdf IntensityCountSlice) {
	counts = p.CountOccurrencesAsSlice()
	cdf = counts.ToCDF()
	return
}

// Create an image from the 2-d histogram, using the supplied function to map
// intensities to colors.
func (p *Hist2D) ToImage(toColor func(v HistCount) color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, p.BucketWidth, p.BucketHeight))
	for row, rowSlice := range p.Data {
		y := p.BucketHeight - row
		for x, v := range rowSlice {
			img.Set(x, y, toColor(v))
		}
	}
	return img
}

// Convert the map of intensity to count into a slice of IntensityCount.
func (m HistCountOccurrences) ToSlice() IntensityCountSlice {
	s := make(IntensityCountSlice, len(m))
	ndx := 0
	for intensity, count := range m {
		s[ndx].Intensity = intensity
		s[ndx].Count = count
		ndx++
	}
	sort.Sort(s)
	return s
}

func (p IntensityCountSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p IntensityCountSlice) Len() int      { return len(p) }
func (p IntensityCountSlice) Less(i, j int) bool {
	return p[i].Intensity < p[j].Intensity
}

// Produce the cumulative density function of this 1-d histogram (i.e.
// each entry's count becomes the sum of all counts of entries with an
// intensity less than or equal to this entry's intensity).
func (p IntensityCountSlice) ToCDF() (cdf IntensityCountSlice) {
	cdf = make(IntensityCountSlice, len(p))
	var cum HistCount
	for ndx := range p {
		cum += p[ndx].Count
		cdf[ndx].Intensity = p[ndx].Intensity
		cdf[ndx].Count = cum
	}
	return
}

/*
	// Find the 1-d histogram bucket that contains intensity (or if not
	// contained, is next to where it should be).
	numIntensities := len(hist1d)
	to1dBucket := func(v geom.HistCount) int {
		// Since most buckets will have a count of zero unless we're very zoomed in
		// or out, let's special case that.
		if v <= hist1d[0].Intensity {
			return 0
		}
		lo, hi := 0, numIntensities - 1
		for lo <= hi {
			mid := (lo + hi) / 2
			i := hist1d[mid].Intensity
			if i < v {
				lo = mid + 1
			} else if i > v {
				hi = mid - 1
			} else {
				return mid
			}
		}
		return lo
	}
*/

func CDFToHEPalette(cdf IntensityCountSlice) (palette color.Palette) {
	cdf_min := cdf[0].Count
	cdf_max := cdf[len(cdf)-1].Count
	denom := float64(cdf_max - cdf_min)
	scale := 255
	palette = make(color.Palette, cdf[len(cdf)-1].Intensity+1)
	var ndx HistCount
	for _, ic := range cdf {
		var c color.NRGBA
		numer := float64(ic.Count - cdf_min)
		if numer > 0 {
			quo := numer / denom
			raw := quo * float64(scale)
			hv := int(math.Floor(raw + 0.5))
			if hv > 255 {
				hv = 255
			}
			c.A = 255
			c.R = uint8(hv)
		}
		for ndx <= ic.Intensity {
			palette[ndx] = c
			ndx++
		}
	}
	return
}

func LogHistogramAndCDF(histogram, cdf IntensityCountSlice) {
	glog.Infof("===== Histogram Table =====")
	glog.Infof("Index Intensity Count Percentage Cumulative%%")
	lastCum := float64(cdf[len(cdf)-1].Count)
	for ndx := range histogram {
		intensity := histogram[ndx].Intensity
		if intensity != cdf[ndx].Intensity {
			glog.Fatalf("ndx=%v  h=%#v  c=%#v", ndx, histogram[ndx], cdf[ndx])
		}
		count := float64(histogram[ndx].Count)
		cum := float64(cdf[ndx].Count)
		glog.Infof("%5d%9d%7d %9.5f%%   %9.5f%%", ndx, intensity, histogram[ndx].Count,
			count/lastCum*100, cum/lastCum*100)
	}
}
