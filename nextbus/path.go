package nextbus

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/jamessynge/transit_tools/geo"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Path struct {
	WayPoints []*Location
	Routes    map[string]*Route
	Hash      uint64
	Index     int
}

func NewPath() *Path {
	return &Path{
		Routes: make(map[string]*Route),
	}
}

func (p *Path) ExtendBounds(min, max *geo.Location) {
	for _, loc := range p.WayPoints {
		if loc.Lat < min.Lat {
			min.Lat = loc.Lat
		} else if loc.Lat > max.Lat {
			max.Lat = loc.Lat
		}
		if loc.Lon < min.Lon {
			min.Lon = loc.Lon
		} else if loc.Lon > max.Lon {
			max.Lon = loc.Lon
		}
	}
	return
}

func (p *Path) Bounds() (min, max geo.Location) {
	min = p.WayPoints[0].Location
	max = min
	p.ExtendBounds(&min, &max)
	return
}

func BoundsOfPaths(paths []*Path) (min, max geo.Location, ok bool) {
	if len(paths) == 0 {
		return
	}
	ok = true
	min, max = paths[0].Bounds()
	for _, path := range paths[1:] {
		path.ExtendBounds(&min, &max)
	}
	return
}

type PathsSlice []*Path

// Len, Less and Swap are the sort.Interface methods.
func (p PathsSlice) Len() int {
	return len(p)
}
func (p PathsSlice) Less(i, j int) bool {
	return p[i].Index < p[j].Index
}
func (p PathsSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p PathsSlice) IsInIndexOrder() bool {
	for i := range p {
		if p[i].Index != i+1 {
			return false
		}
	}
	return true
}

func ToGeoLocations(points []*PointElement, errors *[]error) []geo.Location {
	var err error
	geoLocations := make([]geo.Location, len(points))
	for i, ptElem := range points {
		geoLocations[i], err = geo.LocationFromFloat64s(ptElem.Lat, ptElem.Lon)
		if err != nil {
			*errors = append(*errors, err)
		}
	}
	return geoLocations
}

func Escape(in string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(in))
	return b.String()
}

func writePath(path *Path, w io.Writer) {
	fmt.Fprintf(w, `	<path index="%d">`, path.Index)
	fmt.Fprintln(w)
	for _, route := range path.Routes {
		fmt.Fprintf(w, `		<route tag="%s" title="%s">`, route.Tag, Escape(route.Title)) // TODO Escape
		fmt.Fprintln(w)
		for _, direction := range route.Directions {
			fmt.Fprintf(w, `			<direction tag="%s" title="%s"/>`, direction.Tag, Escape(direction.Title)) // TODO Escape
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, `		</route>`)
	}
	for _, location := range path.WayPoints {
		fmt.Fprintf(w, `		<point lat="%s" lon="%s"/>`,
			strings.TrimSpace(strconv.FormatFloat(float64(location.Lat), 'f', -1, 64)),
			strings.TrimSpace(strconv.FormatFloat(float64(location.Lon), 'f', -1, 64)))
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, `	</path>`)
}

func WritePaths(agency *Agency, w io.Writer) {
	paths := PathsSlice(agency.GetPaths())
	sort.Sort(paths)
	if !sort.IsSorted(paths) || !paths.IsInIndexOrder() {
		log.Printf("paths is not sorted!")
	}

	fmt.Fprintln(w, `<?xml version="1.0" encoding="utf-8" ?>`)
	fmt.Fprintf(w, `<body agency="%s">`, agency.Tag)
	fmt.Fprintln(w)
	for _, path := range paths {
		writePath(path, w)
	}
	fmt.Fprintln(w, `</body>`)
}

func WritePathsToFile(agency *Agency, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	b := bufio.NewWriter(f)
	defer b.Flush()
	WritePaths(agency, b)
	log.Printf("Wrote paths to %s", filePath)
	return nil
}

/*
	decoder := xml.NewDecoder(r)
	for {
		t, err := decoder.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		log.Printf("Token -> %v", t)
		switch e := t.(type) {
		case xml.StartElement:
			if e.Name.Local == "body" {
				if agency != nil {
					log.Fatal("Already found body element!")
				}
				e.
				agency = NewAgency
				vehicle_elements = appendStartElement(vehicle_elements, &e)
				attr := getAttr(e, "secsSinceReport")
				if attr != nil {
					secs, err := strconv.ParseUint(*attr, 10, 16)
					if err == nil {
						if youngestAge == -1 || youngestAge > int(secs) {
							youngestAge = int(secs)
						}
					}
				}
			} else if e.Name.Local == "lastTime" {
				av := getAttr(e, "time")
				if av == nil {
					return nil, fmt.Errorf("time attribute missing: %v", e)
				}
				lastTime, err = strconv.ParseInt(*av, 10, 64)
				if err != nil || lastTime <= 0 {
					return nil, fmt.Errorf("invalid time attribute: %v\nerr: %v", e, err)
				}
			}
		}
	}

*/

type pathsPathElement struct {
	Routes []*RouteElement `xml:"route"`
	Points []*PointElement `xml:"point"`
}
type pathsBodyElement struct {
	AgencyTag string              `xml:"agency,attr"`
	Paths     []*pathsPathElement `xml:"path"`
}

func ReadPaths(r io.Reader) (agency *Agency, err error) {

	// Using xml.Decoder.Decode instead of xml.Unmarshal so that we don't
	// have to read the entire file into memory before decoding.
	decoder := xml.NewDecoder(r)
	body := &pathsBodyElement{}
	err = decoder.Decode(&body)
	if err != nil {
		return
	}
	if len(body.AgencyTag) == 0 || len(body.Paths) == 0 {
		err = fmt.Errorf("Missing required data")
		return
	}
	agency = NewAgency(body.AgencyTag)
	var errors []error
	for _, elem := range body.Paths {
		geoLocations := ToGeoLocations(elem.Points, &errors)
		if geoLocations == nil {
			continue
		}
		path := agency.getOrAddPathByLocations(geoLocations)
		for _, routeElem := range elem.Routes {
			route := agency.getOrAddRouteByTag(routeElem.Tag)
			if route == nil {
				continue
			}
			err = route.initFromElement(routeElem)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			path.Routes[route.Tag] = route
			route.Paths = append(route.Paths, path)
			for _, dirElem := range routeElem.Directions {
				direction := route.getOrAddDirectionByTag(dirElem.Tag)
				err = direction.initFromElement(dirElem)
				if err != nil {
					errors = append(errors, err)
				}
			}
		}
	}
	return
}

func ReadPathsFromFile(filePath string) (agency *Agency, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()
	r := bufio.NewReader(file)
	agency, err = ReadPaths(r)
	return
}
