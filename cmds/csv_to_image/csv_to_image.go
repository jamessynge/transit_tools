package main

import (
	"flag"
	//	"fmt"
	"encoding/csv"
	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
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
func (p *LocationCensus) AddVehicleLocation(vl *nextbus.VehicleLocation) {
	p.AddLocation(vl.Location)
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
	log.Printf("===== Histogram Table =====")
	log.Printf("Index Intensity Count Percentage Cumulative%%")
	lastCum := float64(cdf[len(cdf)-1].Count)
	for ndx := range histogram {
		intensity := histogram[ndx].Intensity
		if intensity != cdf[ndx].Intensity {
			log.Fatalf("ndx=%v  h=%#v  c=%#v", ndx, histogram[ndx], cdf[ndx])
		}
		count := float64(histogram[ndx].Count)
		cum := float64(cdf[ndx].Count)
		log.Printf("%5d%9d%7d %9.5f%%   %9.5f%%", ndx, intensity, histogram[ndx].Count,
			count/lastCum*100, cum/lastCum*100)
	}
}

func AddLocationsFileToCensus(filePath string, census *LocationCensus) (err error) {
	// Open the file for reading.
	rc, err := util.OpenReadFile(filePath)
	if err != nil {
		return
	}
	defer func() {
		err2 := rc.Close()
		if err == nil {
			err = err2
		}
	}()
	cr := csv.NewReader(rc)
	numRecords := 0
	var record []string
	for {
		record, err = cr.Read()
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		numRecords++
		vl, err := nextbus.CSVFieldsToVehicleLocation(record)
		if err != nil {
			log.Printf("Error parsing field location record %d: %v\n\terr: %v",
				numRecords, record, err)
			// TODO Could add an error output channel/file to make it easier to
			// debug later.
			continue
		}
		census.AddVehicleLocation(vl)
	}
	log.Printf("Processed %d records from: %s", numRecords, filePath)
	return
}

var (
	minLatFlag = flag.Float64(
		"min-lat", 42.10,
		"Minimum Latitude (e.g. bottom of edge of image)")
	maxLatFlag = flag.Float64(
		"max-lat", 42.59,
		"Maximum Latitude (e.g. top of edge of image)")
	minLonFlag = flag.Float64(
		"min-lon", -71.30,
		"Minimum Longitude (e.g. left of edge of image)")
	maxLonFlag = flag.Float64(
		"max-lon", -70.84,
		"Maximum Longitude (e.g. right of edge of image)")

	widthFlag = flag.Int(
		"width", 256,
		"Width of image")
	heightFlag = flag.Int(
		"height", 256,
		"Height of image")

	locationsGlobFlag = flag.String(
		"locations", "",
		"Path (glob) of locations csv file(s) to process")

	//	routeTagFlag = flag.String(
	//		"route", "",
	//		"Path (glob) of locations csv file(s) to process")
	//	directionFlag = flag.String(
	//		"locations", "",
	//		"Path (glob) of locations csv file(s) to process")

	outputImgFlag = flag.String(
		"output", "",
		"Image file to write")
)

func main() {
	// Validate args.
	flag.Parse()
	ok := true

	// Are they set?
	minLat, err := geo.LatitudeFromFloat64(*minLatFlag)
	if err != nil {
		ok = false
		log.Printf("--min-lat invalid: %v", err)
	}
	maxLat, err := geo.LatitudeFromFloat64(*maxLatFlag)
	if err != nil {
		ok = false
		log.Printf("--max-lat invalid: %v", err)
	}
	minLon, err := geo.LongitudeFromFloat64(*minLonFlag)
	if err != nil {
		ok = false
		log.Printf("--min-lon invalid: %v", err)
	}
	maxLon, err := geo.LongitudeFromFloat64(*maxLonFlag)
	if err != nil {
		ok = false
		log.Printf("--max-lon invalid: %v", err)
	}

	if len(*outputImgFlag) == 0 {
		ok = false
		log.Print("--output not set")
	} else if util.Exists(*outputImgFlag) && !util.IsFile(*outputImgFlag) {
		ok = false
		log.Printf("Not a file: %v", *outputImgFlag)
	}

	var matchingLocationFilePaths []string
	if len(*locationsGlobFlag) == 0 {
		ok = false
		log.Print("--locations not set")
	} else if ok {
		matchingLocationFilePaths, err = filepath.Glob(*locationsGlobFlag)
		if err != nil {
			ok = false
			log.Printf("Invalid --locations: %v", err)
		} else if len(matchingLocationFilePaths) == 0 {
			if util.IsFile(*locationsGlobFlag) {
				matchingLocationFilePaths =
					append(matchingLocationFilePaths, *locationsGlobFlag)
			} else {
				ok = false
				log.Printf("--locations matched no files: %s", *locationsGlobFlag)
			}
		}
	}

	if !ok {
		flag.PrintDefaults()
		return
	}

	minLoc := geo.Location{minLat, minLon}
	maxLoc := geo.Location{maxLat, maxLon}
	census := NewLocationCensus(minLoc, maxLoc, *widthFlag, *heightFlag)

	for _, filePath := range matchingLocationFilePaths {
		err = AddLocationsFileToCensus(filePath, census)
	}

	log.Printf("NumNonZero: %d (%.2f %%)",
		census.NumNonZero,
		float64(census.NumNonZero)/float64(census.TotalSlots()))
	log.Printf("MaxVal: %d", census.MaxVal)

	histogram, cdf := census.HistogramAndCDF()
	log.Printf("Histogram len: %d", len(histogram))
	log.Printf("      CDF len: %d", len(cdf))
	if len(histogram) != len(cdf) {
		log.Fatalf("Lengths should be the same")
	}
	LogHistogramAndCDF(histogram, cdf)

	palette := CDFToHEPalette(cdf)
	img := ToImage(census, palette)

	f, err := os.Create(*outputImgFlag)
	if err != nil {
		log.Fatal(err)
	}

	err = png.Encode(f, img)
	if err != nil {
		log.Fatal(err)
	}

	err = f.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Wrote to %v", *outputImgFlag)
}
