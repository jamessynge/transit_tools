package nballpaths

// Support for reading (and eventually writing) all-paths.xml files.

import (
	"encoding/xml"
	"fmt"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/nextbus"
)

/*
TODO Add nearest stop tag and title to points, if sufficiently close (sometimes
     it appears to be a rounding error that keeps the lat-lon of the stop from
     being the same as the point lat-lon).  This is intended to support showing
     the stops on outputs of path fitting.

<?xml version="1.0" encoding="utf-8" ?>
<body agency="mbta">
	<path index="1">
		<route tag="1" title="1">
			<direction tag="1_0_var0" title="Harvard Station via Mass. Ave."/>
			<direction tag="1_1_var0" title="Dudley Station via Mass. Ave."/>
		</route>
		<point lat="42.33232" lon="-71.08125"/>
		<point lat="42.33244" lon="-71.08125"/>
	</path>
	<path index="5">
		<route tag="1" title="1">
			<direction tag="1_0_var0" title="Harvard Station via Mass. Ave."/>
			<direction tag="1_1_var0" title="Dudley Station via Mass. Ave."/>
		</route>
		<route tag="701" title="Ct1">
			<direction tag="701_0_var0" title="Central Square (Limited Stops)"/>
			<direction tag="701_1_var0" title="Boston Medical Center (Limited Stops)"/>
		</route>
		<point lat="42.3589399" lon="-71.09363"/>
		<point lat="42.35888" lon="-71.09348"/>
	</path>
</body>
*/

type DirectionElement struct {
	Tag   string `xml:"tag,attr"`
	Title string `xml:"title,attr"`
}
type RouteElement struct {
	Tag        string              `xml:"tag,attr"`
	Title      string              `xml:"title,attr"`
	Directions []*DirectionElement `xml:"direction"`
}
type PointElement struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
}
type PathElement struct {
	Routes []*RouteElement `xml:"route"`
	Points []*PointElement `xml:"point"`
}
type BodyElement struct {
	Agency string         `xml:"agency,attr"`
	Paths  []*PathElement `xml:"route"`
}

func UnmarshalAllPathsBytes(data []byte) (*BodyElement, error) {
	body := &BodyElement{}
	err := xml.Unmarshal([]byte(data), body)
	if err != nil {
		glog.Errorf("Unmarshal returned error: %s", err)
		glog.Errorf("Raw data: %q", data)
		glog.Errorf("Partially decoded: %#v", body)
		return nil, err
	}
	return body, nil
}

func UnmarshalAllPathsReader(r io.Reader) (*BodyElement, error) {
	// Using xml.Decoder.Decode instead of xml.Unmarshal so that we don't
	// have to read the entire file into memory before decoding.
	decoder := xml.NewDecoder(r)
	body := &BodyElement{}
	return body, decoder.Decode(&body)
}

func CreateAgency(body *BodyElement) (*nextbus.Agency, error) {
	if len(body.AgencyTag) == 0 || len(body.Paths) == 0 {
		return nil, fmt.Errorf("Missing required data")
	}
	agency = nextbus.NewAgency(body.AgencyTag)
	errs := util.NewErrors()
	for _, elem := range body.Paths {
		geoLocations := nextbus.ToGeoLocations(elem.Points, &errors)
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

func ReadAllPathsFromFile(filePath string) (agency *Agency, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()
	r := bufio.NewReader(file)
	agency, err = ReadPaths(r)
	return
}
