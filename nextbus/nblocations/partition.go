package nblocations

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/util"
)

var (
	entriesCount  int = 0
	entriesPrefix     = " > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > > "
)

func enterExitInfof(v glog.Level, level uint, format string, args ...interface{}) func() {
	entriesCount++
	if entriesCount < 0 {
		glog.Fatalf("entriesCount (%d) is < 0!!!", entriesCount)
	}
	prefix := entriesPrefix[0 : 2*entriesCount]
	glog.V(v).Infof(prefix+format+" Enter", args...)
	return func() {
		glog.V(v).Infof(prefix+format+" Exit", args...)
		entriesCount--
	}
}

type Partitioner interface {
	OpenForWriting(dir string, delExisting bool) error
	Partition(ral *RecordAndLocation) error
	Close()
	Region() (west, east geo.Longitude, south, north geo.Latitude)
	FileNamesForRegion(west, east geo.Longitude, south, north geo.Latitude) []string
	generateJson(prefix string, w io.Writer)
}
type decoderPartitioner interface {
	Partitioner
	//	decodeRegion(d *Decoder, name string)
	// decodeXml consumes the tokens for "this" type of element (must be on
	// the appropriate StartElement token); when it returns it has advanced
	// past the corresponding EndElement.
	decodeXml(d *Decoder, level uint)
}

type NewDecoderPartitionerFn func() decoderPartitioner

const kAgencyPartitions = "AgencyPartitions"
const kSouthNorthPartitions = "SouthNorthPartitions"
const kWestEastPartitions = "WestEastPartitions"
const kLeafPartition = "LeafPartition"

var (
	newPartitionerFns = map[string]NewDecoderPartitionerFn{
		//		"AgencyPartitions": func() decoderPartitioner { return new(AgencyPartitioner) },
		kSouthNorthPartitions: func() decoderPartitioner { return new(SouthNorthPartitioners) },
		kWestEastPartitions:   func() decoderPartitioner { return new(WestEastPartitioners) },
		kLeafPartition:        func() decoderPartitioner { return new(LeafPartitioner) },
	}
)

func decodeSubPartitioner(d *Decoder, level uint) decoderPartitioner {
	defer enterExitInfof(1, level, "decodeSubPartitioner @ %s", d)()

	if d.err != nil || d.t == nil {
		return nil
	}
	e, ok := d.t.(xml.StartElement)
	if !ok || e.Name.Space != "" {
		return nil
	}
	fn := newPartitionerFns[e.Name.Local]
	glog.V(1).Infof("new function for %s is %v", e.Name.Local, fn)
	if fn == nil {
		return nil
	}
	kid := fn()
	kid.decodeXml(d, level)
	return kid
}

type AgencyPartitioner struct {
	Agency          string `xml:",attr"`
	RootPartitioner Partitioner
	XMLName         xml.Name `xml:"AgencyPartitions"`
}

func (p *AgencyPartitioner) OpenForWriting(dir string, delExisting bool) error {
	return p.RootPartitioner.OpenForWriting(dir, delExisting)
}
func (p *AgencyPartitioner) Partition(ral *RecordAndLocation) error {
	return p.RootPartitioner.Partition(ral)
}
func (p *AgencyPartitioner) Close() {
	p.RootPartitioner.Close()
}
func (p *AgencyPartitioner) Region() (west, east geo.Longitude,
	south, north geo.Latitude) {
	return p.RootPartitioner.Region()
}
func (p *AgencyPartitioner) FileNamesForRegion(
	west, east geo.Longitude, south, north geo.Latitude) (names []string) {
	return p.RootPartitioner.FileNamesForRegion(west, east, south, north)
}
func (p *AgencyPartitioner) decodeXml(d *Decoder, level uint) {
	defer enterExitInfof(1, level, "AgencyPartitioner.decodeXml")()
	d.RequireIsStart(kAgencyPartitions)
	p.Agency = d.GetAttributeValue(kAgencyPartitions, "Agency")
	d.Advance()
	p.RootPartitioner = decodeSubPartitioner(d, 0)
	if p.RootPartitioner == nil {
		glog.Fatalf("Expected a non-leaf Partitioner element, not: %s", d)
	}
	d.RequireIsEnd(kAgencyPartitions)
	d.Next()
	if d.err != io.EOF {
		glog.Fatalf("Expected EOF, not %s", d)
	}
}
func (p *AgencyPartitioner) generateJson(prefix string, w io.Writer) {
	p.RootPartitioner.generateJson("", w)
}

type RegionBase struct {
	West  geo.Longitude `xml:",attr"`
	East  geo.Longitude `xml:",attr"`
	South geo.Latitude  `xml:",attr"`
	North geo.Latitude  `xml:",attr"`
}

func (p *RegionBase) Region() (
	west, east geo.Longitude, south, north geo.Latitude) {
	return p.West, p.East, p.South, p.North
}
func (p *RegionBase) decodeRegion(d *Decoder, level uint, name string) {
	defer enterExitInfof(1, level, "%s.decodeRegion", name)()
	d.RequireIsStart(name)
	var err error
	p.West, err = geo.ParseLongitude(d.GetAttributeValue(name, "West"))
	if err != nil {
		glog.Fatalf("Unable to parse West: %s", err)
	}
	p.East, err = geo.ParseLongitude(d.GetAttributeValue(name, "East"))
	if err != nil {
		glog.Fatalf("Unable to parse East: %s", err)
	}
	p.South, err = geo.ParseLatitude(d.GetAttributeValue(name, "South"))
	if err != nil {
		glog.Fatalf("Unable to parse South: %s", err)
	}
	p.North, err = geo.ParseLatitude(d.GetAttributeValue(name, "North"))
	if err != nil {
		glog.Fatalf("Unable to parse South: %s", err)
	}
}

type PartitionsBase struct {
	RegionBase
	SubPartitions []Partitioner
	minCoord      float64
	maxCoord      float64
	cutPoints     []float64
	level         uint // 0-based, <= *partitionsLevelsFlag
}

func (p *PartitionsBase) OpenForWriting(dir string, delExisting bool) error {
	errs := util.NewErrors()
	for _, sp := range p.SubPartitions {
		errs.AddError(sp.OpenForWriting(dir, delExisting))
	}
	return errs.ToError()
}
func (p *PartitionsBase) partition(
	coord float64, ral *RecordAndLocation) error {
	for n, v := range p.cutPoints {
		if coord < v {
			return p.SubPartitions[n].Partition(ral)
		}
	}
	return p.SubPartitions[len(p.SubPartitions)-1].Partition(ral)
}
func (p *PartitionsBase) Close() {
	for _, sp := range p.SubPartitions {
		sp.Close()
	}
}
func (p *PartitionsBase) decodeBase(d *Decoder, level uint, name string) {
	defer enterExitInfof(1, level, "%s.decodeBase", name)()
	p.decodeRegion(d, level, name)
	d.Advance() // Advance past start element.
	for {
		if d.IsEnd(name) {
			d.Advance()
			return
		}
		kid := decodeSubPartitioner(d, 0)
		p.SubPartitions = append(p.SubPartitions, kid)
	}
}
func (p *PartitionsBase) FileNamesForRegion(
	west, east geo.Longitude, south, north geo.Latitude) (names []string) {
	if west > p.East {
		return
	}
	if east < p.West {
		return
	}
	if south > p.North {
		return
	}
	if north < p.South {
		return
	}
	for _, sp := range p.SubPartitions {
		n2 := sp.FileNamesForRegion(west, east, south, north)
		if len(n2) > 0 {
			names = append(names, n2...)
		}
	}
	return
}
func (p *PartitionsBase) generateJson(prefix string, w io.Writer) {
	if p.level < 2 {
		p.SubPartitions[1].generateJson(prefix, w)
		return
	}
	fmt.Fprintf(w, "%s{\n", prefix)
	pre := prefix + "\t"
	fmt.Fprintf(w, "%ssw: [%v, %v],\n", pre, p.South, p.West)
	fmt.Fprintf(w, "%sne: [%v, %v],\n", pre, p.North, p.East)
	if len(p.SubPartitions) > 0 {
		fmt.Fprintf(w, "%ssubPartitions: [\n", pre)
		for _, sp := range p.SubPartitions {
			sp.generateJson(pre+"\t", w)
			fmt.Fprintln(w, ",")
		}
		fmt.Fprintf(w, "%s],\n", pre)
	}
	fmt.Fprintf(w, "%s}", prefix)
}

////////////////////////////////////////////////////////////////////////////////

type SouthNorthPartitioners struct {
	PartitionsBase
	XMLName xml.Name `xml:"SouthNorthPartitions"`
}

func CreateSouthNorthPartitioners(
	loLon, hiLon geo.Longitude,
	loLat, hiLat geo.Latitude,
	level, levelLimit uint, cutsPerLevel int,
	sortedLocations []geo.Location,
	sortedWestToEast bool) decoderPartitioner {
	if level >= levelLimit {
		return CreateLeafPartitioner(loLon, hiLon, loLat, hiLat, level)
	}

	snLocations := sortedLocations
	sortedLocations = nil
	if sortedWestToEast {
		geo.SortSouthToNorth(snLocations)
	}

	glog.Infof("%s CreateSouthNorthPartitioners(%v, %v, %v, %v, %d, len=%d)",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, len(snLocations))
	if len(snLocations) < int(cutsPerLevel)*10 {
		glog.Fatalf("Too few locations (%d) in S-N partitions at level %d",
			len(snLocations), level)
	}
	p := &SouthNorthPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  loLon,
				East:  hiLon,
				South: loLat,
				North: hiLat,
			},
			minCoord: float64(loLat),
			maxCoord: float64(hiLat),
			level:    level,
		},
	}

	if level < 2 {
		// Special handling of the first two levels, where we always do 2 cuts
		// producing 3 partitions at each level; the outer two partitions at
		// these levels will be leaves, yet will basically just contain a few
		// points that are outliers. The single other non-leaf will cover
		// essentially all of the transit region.
		if loLat != -90 || hiLat != 90 {
			glog.Fatalf("Latitude expected to be full 180, not %v to %v",
				loLat, hiLat)
		}

		southern, northern := getLocationsNearExtrema(snLocations)
		locationsLoLat := southern.Lat
		locationsHiLat := northern.Lat
		glog.Infof("special latitude range: %v to %v", locationsLoLat, locationsHiLat)

		// Remove the locations that are outside that latitude range.
		for n := range snLocations {
			if locationsLoLat > snLocations[n].Lat {
				continue
			}
			if n > 0 {
				snLocations = snLocations[n:]
			}
			break
		}
		for n := len(snLocations); true; {
			n--
			if locationsHiLat < snLocations[n].Lat {
				continue
			}
			snLocations = snLocations[0 : n+1]
			break
		}

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(loLon, hiLon, loLat, locationsLoLat, level+1))

		p.cutPoints = append(p.cutPoints, float64(locationsLoLat))

		if level == 0 {
			p.SubPartitions = append(p.SubPartitions,
				CreateWestEastPartitioners(
					loLon, hiLon, locationsLoLat, locationsHiLat,
					level+1, levelLimit, cutsPerLevel, snLocations, false))
		} else {
			p.SubPartitions = append(p.SubPartitions,
				createPartitioner(
					loLon, hiLon, locationsLoLat, locationsHiLat,
					level+1, levelLimit, cutsPerLevel, snLocations, false))
		}

		p.cutPoints = append(p.cutPoints, float64(locationsHiLat))

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(loLon, hiLon, locationsHiLat, hiLat, level+1))
		return p
	}

	// Normal case (i.e. not including the poles or international dateline).
	// The loop produces the first cutsPerLevel partitions, and the code after
	// the loop produces the last partition.

	minCoord := loLat
	for c := cutsPerLevel + 1; c > 1; c-- {
		// Take the first n locations (this partition) out of the s-n sorted
		// locations, and use them to create a sub-partitioner.
		n := len(snLocations) / int(c)
		partitionLocations := snLocations[0:n]

		// The remaining locations are for the remaining partition(s);
		// the first remaining location defines the Latitude limit (beyond)
		// for this partition.
		snLocations = snLocations[n:]
		maxCoord := snLocations[0].Lat

		// Create a sub-partition based on weRecords.
		sp := createPartitioner(
			loLon, hiLon, minCoord, maxCoord,
			level+1, levelLimit, cutsPerLevel,
			partitionLocations, false)

		p.cutPoints = append(p.cutPoints, float64(maxCoord))
		p.SubPartitions = append(p.SubPartitions, sp)

		minCoord = maxCoord
	}

	// Remaining locations are for the final partition.
	sp := createPartitioner(
		loLon, hiLon, minCoord, hiLat,
		level+1, levelLimit, cutsPerLevel,
		snLocations, false)
	p.SubPartitions = append(p.SubPartitions, sp)

	return p
}
func (p *SouthNorthPartitioners) Partition(
	ral *RecordAndLocation) error {
	return p.partition(float64(ral.Lat), ral)
}
func (p *SouthNorthPartitioners) decodeXml(d *Decoder, level uint) {
	defer enterExitInfof(1, level, "SouthNorthPartitioners.decodeXml")()
	p.level = level
	p.decodeBase(d, level, kSouthNorthPartitions)
	for n, sp := range p.SubPartitions {
		if n != 0 {
			var south, north geo.Latitude
			var w1, w2, e1, e2 geo.Longitude
			w1, e1, south, _ = sp.Region()
			p.cutPoints = append(p.cutPoints, float64(south))
			w2, e2, _, north = p.SubPartitions[n-1].Region()
			if south != north {
				glog.Errorf("level %d, expected latitudes to be equal: %v != %v",
					level, south, north)
			}
			if w1 != w2 {
				glog.Errorf("level %d, expected west longitudes to be equal: %v != %v",
					level, w1, w2)
			}
			if e1 != e2 {
				glog.Errorf("level %d, expected east longitudes to be equal: %v != %v",
					level, e1, e2)
			}
		}
	}
	p.minCoord = float64(p.South)
	p.maxCoord = float64(p.North)
}

////////////////////////////////////////////////////////////////////////////////

type WestEastPartitioners struct {
	PartitionsBase
	XMLName xml.Name `xml:"WestEastPartitions"`
}

func CreateWestEastPartitioners(
	loLon, hiLon geo.Longitude,
	loLat, hiLat geo.Latitude,
	level, levelLimit uint, cutsPerLevel int,
	sortedLocations []geo.Location,
	sortedWestToEast bool) decoderPartitioner {
	if level >= levelLimit {
		return CreateLeafPartitioner(loLon, hiLon, loLat, hiLat, level)
	}

	weLocations := sortedLocations
	sortedLocations = nil
	if !sortedWestToEast {
		geo.SortWestToEast(weLocations)
	}

	glog.Infof("%s CreateWestEastPartitioners(%v, %v, %v, %v, %d, len=%d)",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, len(weLocations))

	if len(weLocations) < cutsPerLevel*10 {
		glog.Fatalf("Too few locations (%d) in W-E partitions at level %d",
			len(weLocations), level)
	}
	p := &WestEastPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  loLon,
				East:  hiLon,
				South: loLat,
				North: hiLat,
			},
			level:    level,
			minCoord: float64(loLon),
			maxCoord: float64(hiLon),
		},
	}
	if level < 2 {
		// Special handling of the first two levels, where we always do 2 cuts
		// producing 3 partitions at each level; the outer two partitions at
		// these levels will be leaves, yet will basically just contain a few
		// points that are outliers. The single other non-leaf will cover
		// essentially all of the transit region.
		if loLon != -180 || hiLon != 180 {
			glog.Fatalf("Longitude expected to be full 360, not %v to %v",
				loLon, hiLon)
		}

		western, eastern := getLocationsNearExtrema(weLocations)
		locationsLoLon := western.Lon
		locationsHiLon := eastern.Lon
		glog.Infof("special longitude range: %v to %v", locationsLoLon, locationsHiLon)

		// Remove the locations that are outside that longitude range.
		for n := range weLocations {
			if locationsLoLon > weLocations[n].Lon {
				continue
			}
			if n > 0 {
				weLocations = weLocations[n:]
			}
			break
		}
		for n := len(weLocations); true; {
			n--
			if locationsHiLon < weLocations[n].Lon {
				continue
			}
			weLocations = weLocations[0 : n+1]
			break
		}

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(loLon, locationsLoLon, loLat, hiLat, level+1))

		p.cutPoints = append(p.cutPoints, float64(locationsLoLon))

		if level == 0 {
			p.SubPartitions = append(p.SubPartitions,
				CreateSouthNorthPartitioners(
					locationsLoLon, locationsHiLon, loLat, hiLat,
					level+1, levelLimit, cutsPerLevel,
					weLocations, true))
		} else {
			p.SubPartitions = append(p.SubPartitions,
				createPartitioner(
					locationsLoLon, locationsHiLon, loLat, hiLat,
					level+1, levelLimit, cutsPerLevel,
					weLocations, true))
		}

		p.cutPoints = append(p.cutPoints, float64(locationsHiLon))

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(locationsHiLon, hiLon, loLat, hiLat, level+1))
		return p
	}

	// Normal case (i.e. not including the poles or international dateline).

	minCoord := loLon
	for c := cutsPerLevel + 1; c > 1; c-- {
		// Take the first n locations (this partition) out of the w-e sorted
		// locations, and use them to create a sub-partitioner.
		n := len(weLocations) / int(c)
		partitionLocations := weLocations[0:n]

		// The remaining w-e locations are for the remaining partitions;
		// the first remaining location defines the Longitude limit (beyond)
		// for this partition.
		weLocations = weLocations[n:]
		maxCoord := weLocations[0].Lon

		// Create a sub-partition based on partitionLocations.
		sp := createPartitioner(
			minCoord, maxCoord, loLat, hiLat,
			level+1, levelLimit, cutsPerLevel,
			partitionLocations, true)

		p.cutPoints = append(p.cutPoints, float64(maxCoord))
		p.SubPartitions = append(p.SubPartitions, sp)

		minCoord = maxCoord
	}

	// Remaining locations are for the final partition.
	sp := createPartitioner(
		minCoord, hiLon, loLat, hiLat,
		level+1, levelLimit, cutsPerLevel,
		weLocations, true)
	p.SubPartitions = append(p.SubPartitions, sp)

	return p
}
func (p *WestEastPartitioners) Partition(
	ral *RecordAndLocation) error {
	return p.partition(float64(ral.Lon), ral)
}
func (p *WestEastPartitioners) decodeXml(d *Decoder, level uint) {
	defer enterExitInfof(1, level, "WestEastPartitioners.decodeXml")()
	p.level = level
	p.decodeBase(d, level, kWestEastPartitions)
	for n, sp := range p.SubPartitions {
		if n != 0 {
			var west, east geo.Longitude
			var s1, s2, n1, n2 geo.Latitude
			west, _, s1, n1 = sp.Region()
			p.cutPoints = append(p.cutPoints, float64(west))
			_, east, s2, n2 = p.SubPartitions[n-1].Region()
			if west != east {
				glog.Errorf("level %d, expected longitudes to be equal: %v != %v",
					level, east, west)
			}
			if s1 != s2 {
				glog.Errorf("level %d, expected south latitudes to be equal: %v != %v",
					level, s1, s2)
			}
			if n1 != n2 {
				glog.Errorf("level %d, expected north latitudes to be equal: %v != %v",
					level, n1, n2)
			}
		}
	}
	p.minCoord = float64(p.West)
	p.maxCoord = float64(p.East)
}

////////////////////////////////////////////////////////////////////////////////

type LeafPartitioner struct {
	XMLName xml.Name `xml:"LeafPartition"`
	RegionBase
	FileName string `xml:",attr"`
	level    uint
	ch       chan *RecordAndLocation
	cwc      *util.CsvWriteCloser
	wg       sync.WaitGroup
}

func CreateLeafPartitioner(
	loLon, hiLon geo.Longitude, loLat, hiLat geo.Latitude,
	level uint) *LeafPartitioner {
	snDistance, weDistance := geo.MeasureCentralAxes(loLon, hiLon, loLat, hiLat)
	glog.Infof(
		"%s CreateLeafPartitioner(%v, %v, %v, %v, %d) S2N: %dm, W2E: %dm",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, int(snDistance), int(weDistance))

	fn := fmt.Sprintf("%s_%s-to-%s_%s.csv.gz",
		loLat.FormatDMS(), loLon.FormatDMS(),
		hiLat.FormatDMS(), hiLon.FormatDMS())
	glog.Infof("           will create %s", fn)

	//		// Add a header identifying the fields to the csv file.
	//		fieldNames := nextbus.VehicleCSVFieldNames()
	//		fieldNames[0] = "# " + fieldNames[0]
	//		p.csv.Write(fieldNames)

	return &LeafPartitioner{
		RegionBase: RegionBase{
			West:  loLon,
			East:  hiLon,
			South: loLat,
			North: hiLat,
		},
		FileName: fn,
		level:    level,
	}
}
func (p *LeafPartitioner) decodeXml(d *Decoder, level uint) {
	defer enterExitInfof(1, level, "LeafPartitioner.decodeXml")()
	p.level = level
	p.decodeRegion(d, level, kLeafPartition)
	p.FileName = d.GetAttributeValue(kLeafPartition, "FileName")
	d.Advance()
	d.RequireIsEnd(kLeafPartition)
	d.Advance()
}
func (p *LeafPartitioner) OpenForWriting(dir string, delExisting bool) (err error) {
	fp := filepath.Join(dir, p.FileName)
	p.cwc, err = util.OpenCsvWriteCloser(fp, true, delExisting, 0644)
	if err != nil {
		return
	}
	p.wg.Add(1)
	p.ch = make(chan *RecordAndLocation, 50)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(time.Minute)
		count := 0
		for {
			var err error
			select {
			case ral, ok := <-p.ch:
				if !ok {
					return
				}
				err = p.cwc.Write(ral.Record)
			case <-ticker.C:
				count++
				if count > 10 {
					err = p.cwc.Flush()
					count = 0
				} else {
					err = p.cwc.PartialFlush()
				}
			}
			if err != nil {
				glog.Warningf("Error from CsvWriteCloser for %s\nError: %s", fp, err)
			}
		}
	}()
	return
}
func (p *LeafPartitioner) Partition(ral *RecordAndLocation) error {
	if p.ch == nil {
		return fmt.Errorf("Must call OpenForWriting for Partition")
	}
	p.ch <- ral
	return nil
}
func (p *LeafPartitioner) Close() {
	if p.ch != nil {
		close(p.ch)
	}
	p.wg.Wait()
	if p.cwc != nil {
		p.cwc.Close()
	}
}
func (p *LeafPartitioner) FileNamesForRegion(
	west, east geo.Longitude, south, north geo.Latitude) (names []string) {
	if west > p.East {
		return
	}
	if east < p.West {
		return
	}
	if south > p.North {
		return
	}
	if north < p.South {
		return
	}
	names = append(names, p.FileName)
	return
}
func (p *LeafPartitioner) generateJson(prefix string, w io.Writer) {
	fmt.Fprintf(w, "%s{\n", prefix)
	pre := prefix + "\t"
	fmt.Fprintf(w, "%sfilename: %q,\n", pre, p.FileName)
	fmt.Fprintf(w, "%ssw: [%v, %v],\n", pre, p.South, p.West)
	fmt.Fprintf(w, "%sne: [%v, %v],\n", pre, p.North, p.East)
	fmt.Fprintf(w, "%s}", prefix)
}

////////////////////////////////////////////////////////////////////////////////

// Choose a direction for partitioning based on the axis with the
// greatest distance (i.e. try to avoid making long thin rectangles).
func createPartitioner(
	loLon, hiLon geo.Longitude,
	loLat, hiLat geo.Latitude,
	level, levelLimit uint, cutsPerLevel int,
	sortedLocations []geo.Location,
	sortedWestToEast bool) decoderPartitioner {
	snDistance, weDistance := geo.MeasureCentralAxes(loLon, hiLon, loLat, hiLat)
	glog.Infof(
		"%s createPartitioner(%v, %v, %v, %v, %d) S2N: %dm, W2E: %dm",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, int(snDistance), int(weDistance))

	if weDistance >= snDistance {
		if weDistance > 500 {
			return CreateWestEastPartitioners(
				loLon, hiLon, loLat, hiLat,
				level, levelLimit, int(cutsPerLevel),
				sortedLocations, sortedWestToEast)
		}
	} else {
		if snDistance > 500 {
			return CreateSouthNorthPartitioners(
				loLon, hiLon, loLat, hiLat,
				level, levelLimit, int(cutsPerLevel),
				sortedLocations, sortedWestToEast)
		}
	}

	return CreateLeafPartitioner(loLon, hiLon, loLat, hiLat, level)
}

func avgOfLocations(locations []geo.Location) (avg geo.Location) {
	for n := range locations {
		avg.Lat += locations[n].Lat
		avg.Lon += locations[n].Lon
	}
	avg.Lat /= geo.Latitude(len(locations))
	avg.Lon /= geo.Longitude(len(locations))
	return
}

// Estimate the latitude and longitude boundaries by averaging values near
// the extrema, but not the extrema because sometimes there are substantial
// errors (1000s of miles, e.g. I've seen a sample in the Los Angeles, CA area
// for the MBTA in MA).
func getLocationsNearExtrema(
	sortedSamples []geo.Location) (
	low, high geo.Location) {
	var lowIndex int
	if len(sortedSamples) >= 1000000 {
		lowIndex = 200
	} else if len(sortedSamples) >= 100000 {
		lowIndex = 50
	} else if len(sortedSamples) >= 10000 {
		lowIndex = 20
	} else {
		glog.Fatalf("Too few locations (%d) for creating partitioner", len(sortedSamples))
	}

	low = avgOfLocations(sortedSamples[lowIndex/2 : lowIndex])

	highIndex := len(sortedSamples) - lowIndex
	high = avgOfLocations(sortedSamples[highIndex : highIndex+lowIndex/2])

	return
}

func CreateAgencyPartitioner(agency string, samples []geo.Location,
	levelLimit uint, cutsPerLevel uint) *AgencyPartitioner {
	if levelLimit < 2 {
		glog.Fatalf("Too few levels (%d)", levelLimit)
	}
	if cutsPerLevel < 1 {
		glog.Fatalf("Too few cuts per level (%d)", cutsPerLevel)
	}
	var partitionsPerLevel int64 = int64(cutsPerLevel) + 1
	var totalLeaves int64 = 1
	for n := uint(0); n < (levelLimit - 2); n++ {
		totalLeaves *= partitionsPerLevel
	}
	totalLeaves += 4 // For the two special leaves at each of the first two levels.
	minSamples := totalLeaves * 100
	if int64(len(samples)) < minSamples {
		glog.Fatalf(
			"Too few location samples (%d); want at least %d samples given %d leaves in the partitioning",
			len(samples), minSamples, totalLeaves)
	}

	// Start the partitioning in the direction with the greatest distance.
	snLocations := append([]geo.Location(nil), samples...)
	geo.SortSouthToNorth(snLocations)

	southern, northern := getLocationsNearExtrema(snLocations)
	glog.Infof("southern position: %s", southern)
	glog.Infof("northern position: %s", northern)

	weLocations := samples
	samples = nil
	geo.SortWestToEast(weLocations)
	western, eastern := getLocationsNearExtrema(weLocations)
	glog.Infof("western position: %s", western)
	glog.Infof("eastern position: %s", eastern)

	var south, north, west, east geo.Location
	south.Lat = southern.Lat
	north.Lat = northern.Lat
	west.Lon = western.Lon
	east.Lon = eastern.Lon

	// Now use the median latitude and longitude for computing the
	// longitude and latitude distances, respectively.
	south.Lon = weLocations[len(weLocations)/2].Lon
	north.Lon = south.Lon
	west.Lat = snLocations[len(snLocations)/2].Lat
	east.Lat = west.Lat

	glog.Infof("south: %s", south)
	glog.Infof("north: %s", north)
	glog.Infof("west: %s", west)
	glog.Infof("east: %s", east)

	// Measure the distance.
	snDistance, _ := geo.ToDistanceAndHeading(south, north)
	weDistance, _ := geo.ToDistanceAndHeading(west, east)

	glog.Infof("snDistance: %v", snDistance)
	glog.Infof("weDistance: %v", weDistance)

	// TODO add func createRootPartitioners to create the 5 partitioners for
	// the two root levels.

	result := &AgencyPartitioner{Agency: agency}
	if weDistance >= snDistance {
		result.RootPartitioner = CreateWestEastPartitioners(
			-180, 180, -90, 90,
			0, levelLimit, int(cutsPerLevel),
			weLocations, true)
	} else {
		result.RootPartitioner = CreateSouthNorthPartitioners(
			-180, 180, -90, 90,
			0, levelLimit, int(cutsPerLevel),
			snLocations, false)
	}

	return result
}

////////////////////////////////////////////////////////////////////////////////

type Decoder struct {
	decoder *xml.Decoder
	t       xml.Token
	err     error
}

func (p *Decoder) String() string {
	if p.err != nil {
		return "Error: " + p.err.Error()
	}
	if p.t == nil {
		return "<nil>"
	}
	pn := func(n xml.Name) string {
		if n.Space == "" {
			return n.Local
		} else {
			return n.Space + ":" + n.Local
		}
	}

	switch e := p.t.(type) {
	case xml.StartElement:
		var b bytes.Buffer
		b.WriteRune('<')
		b.WriteString(pn(e.Name))
		for _, attr := range e.Attr {
			b.WriteRune(' ')
			b.WriteString(pn(attr.Name))
			b.WriteRune('=')
			fmt.Fprintf(&b, "%q", attr.Value)
		}
		b.WriteRune('>')
		return b.String()
	case xml.EndElement:
		return fmt.Sprintf("</%s>", pn(e.Name))
	}

	return fmt.Sprintf("%#v", p.t)
}

func (p *Decoder) Next() bool {
	for {
		if p.t, p.err = p.decoder.Token(); p.err != nil {
			return false
		} else {
			switch p.t.(type) {
			case xml.StartElement:
				return true
			case xml.EndElement:
				return true
			case xml.CharData:
				c := p.t.(xml.CharData)
				if len(strings.TrimSpace(string(c))) > 0 {
					//					return true
					glog.Fatalf("Unexpected chardata: %s", string(c))
				}
			}
		}
	}
}
func (p *Decoder) Advance() {
	if !p.Next() {
		glog.Fatalf("Unexpected error: %s", p.err)
	}
	glog.V(1).Infoln("Advanced to", p)
}
func (p *Decoder) IsStart(name string) bool {
	if p.err != nil || p.t == nil {
		return false
	}
	e, ok := p.t.(xml.StartElement)
	return ok && (name == "" || e.Name.Local == name)
}
func (p *Decoder) RequireIsStart(name string) {
	if !p.IsStart(name) {
		glog.Fatalf("Expected start of element %s, not %s", name, p)
	}
}
func (p *Decoder) GetStart(name string) xml.StartElement {
	p.RequireIsStart(name)
	return p.t.(xml.StartElement)
}
func (p *Decoder) IsEnd(name string) bool {
	if p.err != nil || p.t == nil {
		return false
	}
	e, ok := p.t.(xml.EndElement)
	return ok && (name == "" || e.Name.Local == name)
}
func (p *Decoder) RequireIsEnd(name string) {
	if !p.IsEnd(name) {
		glog.Fatalf("Expected end of element %s, not %s", name, p)
	}
}

//func (p *Decoder) IsCharData() bool {
//	_, ok := p.t.(xml.CharData)
//	return ok
//}
//func (p *Decoder) GetCharData() bool {
//	return p.t.(xml.CharData)
//}
func (p *Decoder) FindAttributeValue(eName, aName string) (string, bool) {
	e := p.GetStart(eName)
	for i := range e.Attr {
		if e.Attr[i].Name.Local == aName {
			return e.Attr[i].Value, true
		}
	}
	return "", false
}
func (p *Decoder) GetAttributeValue(eName, aName string) string {
	e := p.GetStart(eName)
	for i := range e.Attr {
		if e.Attr[i].Name.Local == aName {
			return e.Attr[i].Value
		}
	}
	glog.Fatalf("Attribute %s missing from: %s", aName, e)
	return ""
}

func UnmarshalAgencyPartitioner(r io.Reader) *AgencyPartitioner {
	d := &Decoder{decoder: xml.NewDecoder(r)}
	d.Advance()
	a := &AgencyPartitioner{}
	a.decodeXml(d, 0)
	d.Next()
	if d.t != nil {
		glog.Warningf("Unexpected token(s) at end of partition index: %s", d)
	}
	return a
}

func GetPartitionsIndexPath(dir string) string {
	return filepath.Join(dir, "partitions.xml")
}

func SavePartitionsIndex(dir string, a *AgencyPartitioner) {
	b, err := xml.MarshalIndent(a, "", " ")
	if err != nil {
		glog.Fatalf("Unable to marshal partitioner: %s", err)
	}
	fp := GetPartitionsIndexPath(dir)
	err = ioutil.WriteFile(fp, b, 0644)
	if err != nil {
		glog.Fatalf("Unable to save marshalled partitioners: %s", err)
	}
	glog.Infof("Saved partitions index to %s", fp)

	b = GenerateHtml(a)
	fp = filepath.Join(dir, "partitions.html")
	err = ioutil.WriteFile(fp, b, 0644)
	if err != nil {
		glog.Fatalf("Unable to save html map of partitions: %s", err)
	}
	glog.Infof("Saved partitions map to %s", fp)
}

func ReadPartitionsIndex(dir string) *AgencyPartitioner {
	fp := GetPartitionsIndexPath(dir)
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		glog.Fatalf("Unable to read file %s\nError: %s", fp, err)
	}
	a := UnmarshalAgencyPartitioner(bytes.NewReader(b))
	//	if err != nil {
	//		glog.Fatalf("Unable to decode file %s\nError: %s", fp, err)
	//	}
	glog.Infof("Restored partition definitions from %s", fp)
	return a
}

////////////////////////////////////////////////////////////////////////////////

const (
	htmlHeader = ` 
<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="initial-scale=1.0, user-scalable=no">
		<meta charset="utf-8">
		<title>Rectangles</title>
		<style>
			html, body, #map-canvas {
				height: 100%;
				margin: 0px;
				padding: 0px
			}
		</style>
		<script src="https://maps.googleapis.com/maps/api/js?v=3.exp"></script>
		<script>
var partitionIndex = null, currentLevel = -1, maxLevel = -1,
		rectangles = [], map = null;
function definePartitions() {
	return (
`
	// Here we insert a js function that provides a function definePartitions()
	// that returns an object.  Call after maps api loaded.
	// Partition object {
	//     filename: ""  (property present only if leaf)
	//     sw: [latitude, longitude]
	//     ne: [latitude, longitude]
	//     subPartitions: array of sub partitions, property absent if leaf.
	//     // Following props added during initialization.
	//     level: integer, 0 based
	//     mapRectangle: google.maps.Rectangle, doesn't have map set until other
	//                   code does so.
	//    }

	htmlTrailer = `
	);
}
function getRandomInt(min, max) {
	return Math.floor(Math.random() * (max - min)) + min;
}
function numberToHexHex(v) {
	var hex = (Number(v) + 0).toString(16);
	if (hex.length == 1) {
		return '0' + hex;
	}
	return hex;
}
function getRandomColor() {
	var r = getRandomInt(128, 256),
			g = getRandomInt(128, 256),
			b = getRandomInt(128, 256);
	return "#" + numberToHexHex(r) + numberToHexHex(g) + numberToHexHex(b);
}
function initPartition(level, partition) {
	partition.level = level;
	if (level > maxLevel) {
		maxLevel = level;
	}
	if (partition.subPartitions) {
		var n;
		for (n = 0; n < partition.subPartitions.length; n++) {
			initPartition(level+1, partition.subPartitions[n]);
		}
	}
	partition.bounds = new google.maps.LatLngBounds(
				new google.maps.LatLng(partition.sw[0], partition.sw[1]),
				new google.maps.LatLng(partition.ne[0], partition.ne[1]));
	partition.mapRectangle = new google.maps.Rectangle({
		strokeColor: getRandomColor(),
		strokeOpacity: 0.8,
		strokeWeight: 2,
		fillColor: getRandomColor(),
		fillOpacity: 0.35,
		map: null,
		bounds: partition.bounds,
	});
}
function drawLevel(level, partition) {
	partition = partition || partitionIndex;
	if (level == partition.level) {
		partition.mapRectangle.setMap(map);
	} else {
		partition.mapRectangle.setMap(null);
	}
	if (partition.subPartitions) {
		var n;
		for (n = 0; n < partition.subPartitions.length; n++) {
			drawLevel(level, partition.subPartitions[n]);
		}
	}
}
function setCurrentLevel(level) {
	if (level < 0) {
		level = 0;
	} else if (level > maxLevel) {
		level = maxLevel;
	}
	if (level != currentLevel) {
		drawLevel(level, partitionIndex);
	}
	currentLevel = level;
}
function onKeyDown(e) {
	e = e || window.event;
	if (e.keyCode == '38') { // up arrow
		setCurrentLevel(currentLevel + 1);
	}
	else if (e.keyCode == '40') { // down arrow
		setCurrentLevel(currentLevel - 1);
	}
	else if (e.keyCode == '37') { // left arrow
	}
	else if (e.keyCode == '39') { // right arrow
	}
}

function initialize() {
	partitionIndex = definePartitions();
	initPartition(0, partitionIndex);
	map = new google.maps.Map(document.getElementById('map-canvas'), {
		zoom: 12,
		center: new google.maps.LatLng(42.427905,-71.20695),
		mapTypeId: google.maps.MapTypeId.ROADMAP,
		keyboardShortcuts: false,
	});
	map.fitBounds(partitionIndex.bounds);
	setCurrentLevel(0);
	document.onkeydown = onKeyDown;
}
google.maps.event.addDomListener(window, 'load', initialize);

		</script>
	</head>
	<body>
		<div id="map-canvas"></div>
	</body>
</html>
`
)

func GenerateHtml(a *AgencyPartitioner) []byte {
	var buf bytes.Buffer
	buf.WriteString(htmlHeader)
	a.generateJson("", &buf)
	buf.WriteString(htmlTrailer)
	return buf.Bytes()
}
