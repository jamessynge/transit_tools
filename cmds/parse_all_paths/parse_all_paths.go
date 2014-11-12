package main

import (
	"fmt"
	"github.com/jamessynge/transit_tools/nextbus"
)

func main() {
	agency, err := nextbus.ReadPathsFromFile(`C:\nextbus\mbta\all-paths.xml`)
	if err != nil {
		fmt.Printf("Error: %#v", err)
		panic(err)
	}
	fmt.Printf("agency tag: %s", agency.Tag)
	fmt.Printf("Parsed %d routes, %d directions, %d stops, %d locations, %d paths\n",
		len(agency.Routes), len(agency.Directions), len(agency.Stops), len(agency.Locations), agency.NumPaths())
}
