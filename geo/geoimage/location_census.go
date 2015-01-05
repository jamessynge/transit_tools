package geoimage

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
)

type LocationCensus struct {
	LatMin     float64
	LonMin     float64
	GeoWidth   float64
	GeoHeight  float64
	Width      int
	Height     int
	Data       [][]uint32
	MaxVal     uint32
	NumNonZero uint32
}

type IntensityCount struct {
	Intensity, Count uint32
}
type IntensityCountSlice []IntensityCount

func NewLocationCensus(min, max geo.Location, width, height int) *LocationCensus {
	if min.Lat > max.Lat {
		min.Lat, max.Lat = max.Lat, min.Lat
	}
	if min.Lon > max.Lon {
		min.Lon, max.Lon = max.Lon, min.Lon
	}
	lc := &LocationCensus{
		LatMin:    float64(min.Lat),
		LonMin:    float64(min.Lon),
		GeoWidth:  float64(max.Lon - min.Lon),
		GeoHeight: float64(max.Lat - min.Lat),
		Width:     width,
		Height:    height,
		Data:      make([][]uint32, height),
	}
	for rowIndex := range lc.Data {
		lc.Data[rowIndex] = make([]uint32, width)
	}
	return lc
}

func (p *LocationCensus) LatitudeToRow(lat geo.Latitude) (row int) {
	relative := float64(lat) - p.LatMin
	frac := relative / p.GeoHeight
	row = int(frac * float64(p.Height))
	return
}
func (p *LocationCensus) LongitudeToColumn(lon geo.Longitude) (column int) {
	relative := float64(lon) - p.LonMin
	frac := relative / p.GeoWidth
	column = int(frac * float64(p.Width))
	return
}
func (p *LocationCensus) IncrementRC(row, column int) {
	if 0 <= row && row < p.Height && 0 <= column && column < p.Width {
		rowSlice := p.Data[row]
		val := rowSlice[column] + 1
		rowSlice[column] = val
		if val == 1 {
			p.NumNonZero++
		}
		if p.MaxVal < val {
			p.MaxVal = val
		}
	}
}
func (p *LocationCensus) AddLocation(loc geo.Location) {
	row := p.LatitudeToRow(loc.Lat)
	column := p.LongitudeToColumn(loc.Lon)
	p.IncrementRC(row, column)
}
func (p *LocationCensus) TotalSlots() int {
	return p.Width * p.Height
}
func (p *LocationCensus) HistogramMap() (histogramMap map[uint32]uint32) {
	histogramMap = make(map[uint32]uint32)
	for _, rowSlice := range p.Data {
		for _, count := range rowSlice {
			histogramMap[count]++
		}
	}
	return
}
func (p *LocationCensus) Histogram() IntensityCountSlice {
	histogramMap := p.HistogramMap()
	return MapToHistogram(histogramMap)
}
func (p *LocationCensus) HistogramAndCDF() (
	histogram, cdf IntensityCountSlice) {
	histogram = p.Histogram()
	cdf = HistogramToCDF(histogram)
	return
}

func (p IntensityCountSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p IntensityCountSlice) Len() int      { return len(p) }
func (p IntensityCountSlice) Less(i, j int) bool {
	return p[i].Intensity < p[j].Intensity
}

func MapToHistogram(histogramMap map[uint32]uint32) (
	histogram IntensityCountSlice) {
	histogram = make(IntensityCountSlice, len(histogramMap))
	var ndx int
	for intensity, count := range histogramMap {
		histogram[ndx] = IntensityCount{intensity, count}
		ndx++
	}
	sort.Sort(histogram)
	return
}
func HistogramToCDF(histogram IntensityCountSlice) (cdf IntensityCountSlice) {
	cdf = make(IntensityCountSlice, len(histogram))
	var cum uint32
	for ndx, ic := range histogram {
		cum = cum + ic.Count
		cdf[ndx] = IntensityCount{ic.Intensity, cum}
	}
	return
}
func CDFToHEPalette(cdf IntensityCountSlice) (palette color.Palette) {
	cdf_min := cdf[0].Count
	cdf_max := cdf[len(cdf)-1].Count
	denom := float64(cdf_max - cdf_min)
	scale := 255
	palette = make(color.Palette, cdf[len(cdf)-1].Intensity+1)
	var ndx uint32
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
func ToImage(census *LocationCensus, palette color.Palette) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, census.Width, census.Height))
	for row, rowSlice := range census.Data {
		for col, count := range rowSlice {
			c := palette[count]
			img.Set(col, census.Width-row, c)
		}
	}
	return img
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
