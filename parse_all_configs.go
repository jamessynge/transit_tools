package main

import (
"flag"
	"fmt"
	"nextbus"
	"util"
)



var (
	agencyFlag = flag.String(
		"agency", "",
		"Name of the transit agency.")
	routeConfigFlag = flag.String(
		"route-config", "",
		"Directory in which to find the route configuration files.")
	allPathsFlag = flag.String(
		"all-paths", "",
		"Path of xml file to write with all paths from route configuration files.")
)

func main() {
	flag.Parse()
	if len(*agencyFlag) == 0 { panic("Need -agency"); }
	if len(*routeConfigFlag) == 0 { panic("Need -route-config"); }
	if len(*allPathsFlag) == 0 { panic("Need -all-paths"); }
	if !util.IsDirectory(*routeConfigFlag) { panic("-route-config must specify a directory"); }

	agency := nextbus.NewAgency(*agencyFlag)
	err := nextbus.ParseRouteConfigsDir(agency, *routeConfigFlag)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Parsed %d routes, %d directions, %d stops, %d locations, %d paths\n",
		len(agency.Routes), len(agency.Directions), len(agency.Stops), len(agency.Locations), agency.NumPaths())
	nextbus.WritePathsToFile(agency, *allPathsFlag)
}
