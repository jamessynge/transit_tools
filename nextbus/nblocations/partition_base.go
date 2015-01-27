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
	defer util.EnterExitVInfof(1, "decodeSubPartitioner @ %s", d)()

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

////////////////////////////////////////////////////////////////////////////////

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
	defer util.EnterExitVInfof(1, "AgencyPartitioner.decodeXml")()
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

////////////////////////////////////////////////////////////////////////////////

type RegionBase struct {
	West  geo.Longitude `xml:",attr"`
	East  geo.Longitude `xml:",attr"`
	South geo.Latitude  `xml:",attr"`
	North geo.Latitude  `xml:",attr"`
}
func (p *RegionBase) String() string {
	return fmt.Sprintf("Region[%s to %s, %s to %s]",
			p.West.FormatDMS(), p.East.FormatDMS(),
			p.South.FormatDMS(), p.North.FormatDMS())
}
func (p *RegionBase) Region() (
	west, east geo.Longitude, south, north geo.Latitude) {
	return p.West, p.East, p.South, p.North
}
func (p *RegionBase) decodeRegion(d *Decoder, level uint, name string) {
	defer util.EnterExitVInfof(1, "%s.decodeRegion", name)()
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

////////////////////////////////////////////////////////////////////////////////

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
	defer util.EnterExitVInfof(1, "%s.decodeBase", name)()
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

func (p *SouthNorthPartitioners) Partition(
	ral *RecordAndLocation) error {
	return p.partition(float64(ral.Lat), ral)
}
func (p *SouthNorthPartitioners) decodeXml(d *Decoder, level uint) {
	defer util.EnterExitVInfof(1, "SouthNorthPartitioners.decodeXml")()
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
func (p *WestEastPartitioners) Partition(
	ral *RecordAndLocation) error {
	return p.partition(float64(ral.Lon), ral)
}
func (p *WestEastPartitioners) decodeXml(d *Decoder, level uint) {
	defer util.EnterExitVInfof(1, "WestEastPartitioners.decodeXml")()
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
func (p *LeafPartitioner) decodeXml(d *Decoder, level uint) {
	defer util.EnterExitVInfof(1, "LeafPartitioner.decodeXml")()
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
				if count >= 10 {
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
// errors (10s of miles, e.g. I've seen a sample in Buzzards Bay of the south
// Massachusetts cost, well away from the area covered by the MBTA).
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

func getIndexOfLatitude(snSortedSamples []geo.Location, v geo.Latitude) int {
	// Binary search to find lowest index of a location whose latitude
	// is at least v.
	lo, hi := 0, len(snSortedSamples)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		curr := snSortedSamples[mid].Lat
		if curr < v {
			lo = mid + 1
		} else if curr >= v {
			hi = mid - 1
		}
	}
	return lo
}

func getIndexOfLongitude(weSortedSamples []geo.Location, v geo.Longitude) int {
	// Binary search to find lowest index of a location whose longitude
	// is at least v.
	lo, hi := 0, len(weSortedSamples)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		curr := weSortedSamples[mid].Lon
		if curr < v {
			lo = mid + 1
		} else if curr >= v {
			hi = mid - 1
		}
	}
	return lo
}

func splitLocationsByIndices(
		samples []geo.Location, cutIndices []int) (slices [][]geo.Location) {
	i := 0
	for _, j := range cutIndices {
		slices = append(slices, samples[i:j])
		i = j
	}
	slices = append(slices, samples[i:len(samples)])
	return
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

////////////////////////////////////////////////////////////////////////////////

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
		<script src="https://maps.googleapis.com/maps/api/js?v=3.exp&libraries=geometry"></script>
		<script>
var partitionIndex, map, label,
		currentLevel = -1, maxLevel = -1, rectangles = [];
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
////////////////////////////////////////////////////////////////////////////////
// From http://blog.mridey.com/2009/09/label-overlay-example-for-google-maps.html
// Define the overlay, derived from google.maps.OverlayView
function Label(opt_options) {
 // Initialization
 this.setValues(opt_options);
 // Label specific
 var span = this.span_ = document.createElement('span');
 span.style.cssText = 'position: relative; left: -50%; top: -8px; ' +
                      'white-space: nowrap; border: 1px solid blue; ' +
                      'padding: 2px; background-color: white';
 var div = this.div_ = document.createElement('div');
 div.appendChild(span);
 div.style.cssText = 'position: absolute; display: none';
};
Label.prototype = new google.maps.OverlayView;
// Implement onAdd
Label.prototype.onAdd = function() {
 var pane = this.getPanes().overlayLayer;
 pane.appendChild(this.div_);
 // Ensures the label is redrawn if the text or position is changed.
 var me = this;
 this.listeners_ = [
   google.maps.event.addListener(this, 'position_changed',
       function() { me.draw(); }),
   google.maps.event.addListener(this, 'text_changed',
       function() { me.draw(); })
 ];
};
// Implement onRemove
Label.prototype.onRemove = function() {
 this.div_.parentNode.removeChild(this.div_);
 // Label is removed from the map, stop updating its position/text.
 for (var i = 0, I = this.listeners_.length; i < I; ++i) {
   google.maps.event.removeListener(this.listeners_[i]);
 }
};
// Implement draw
Label.prototype.draw = function() {
 var projection = this.getProjection();
 var position = projection.fromLatLngToDivPixel(this.get('position'));
 var div = this.div_;
 div.style.left = position.x + 'px';
 div.style.top = position.y + 'px';
 div.style.display = 'block';
 this.span_.innerHTML = this.get('text').toString();
};
////////////////////////////////////////////////////////////////////////////////
function arrayToLatLng(pos) {
  return new google.maps.LatLng(pos[0], pos[1]);
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
  var sw = partition.sw, ne = partition.ne,
      se = [sw[0], ne[1]], nw = [ne[0], sw[1]];
  sw = arrayToLatLng(sw); // console.log("sw: " + sw.toString());
  ne = arrayToLatLng(ne); // console.log("ne: " + ne.toString());
  se = arrayToLatLng(se); // console.log("se: " + se.toString());
  nw = arrayToLatLng(nw); // console.log("nw: " + nw.toString());
  partition.bounds = new google.maps.LatLngBounds(sw, ne);
  partition.mapRectangle = new google.maps.Rectangle({
    strokeColor: getRandomColor(),
    strokeOpacity: 0.8,
    strokeWeight: 2,
    fillColor: getRandomColor(),
    fillOpacity: 0.35,
    map: null,
    bounds: partition.bounds,
  });
  partition.area = (
      google.maps.geometry.spherical.computeArea([ne, nw, sw, se]) / 1000000);
  if (partition.area > 50) {
    partition.text = partition.area.toFixed(0) + " sq km";
  } else if (partition.area > 10) {
    partition.text = partition.area.toFixed(1) + " sq km";
  } else if (partition.area > 1) {
    partition.text = partition.area.toFixed(2) + " sq km";
  } else {
    partition.text = partition.area.toFixed(3) + " sq km";
  }
  google.maps.event.addListener(
    partition.mapRectangle,
    "mousemove",
    function(event) {
      label.set('position', event.latLng);
      label.set('text', partition.text);
      label.setMap(map);
    });
  google.maps.event.addListener(
    partition.mapRectangle,
    "mouseout",
    function(event) {
      label.setMap(null);
    });
}
function drawLevel(level, partition) {
  partition = partition || partitionIndex;
  if (level == partition.level) {
    partition.mapRectangle.setMap(map);
  } else if (level > partition.level && !partition.subPartitions) {
  	// If it has no children, show it instead of nothing.
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
    label.setMap(null);
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
  map = new google.maps.Map(document.getElementById('map-canvas'), {
    zoom: 12,
    center: new google.maps.LatLng(42.427905,-71.20695),
    mapTypeId: google.maps.MapTypeId.ROADMAP,
    keyboardShortcuts: false,
  });
  label = new Label();
  initPartition(0, partitionIndex);
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
