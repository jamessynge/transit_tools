package main

/*
c:
cd \Users\james\Documents\EclipseProjects\go\src\github.com\jamessynge\transit_tools
go run cmds\locations_to_nearest_paths\locations_to_nearest_paths.go --alsologtostderr --locations=X:\mbta\locations\processed\2014\*\*.csv.gz --output=c:\temp\locations-by-path\ --all-paths=c:\temp\locations-by-path\all-paths.xml --overwrite
*/

import (
	"flag"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang/glog"

	//	"github.com/jamessynge/transit_tools/geom"
	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/nextbus/nbgeo"
	"github.com/jamessynge/transit_tools/util"
)

// The flag package provides a default help printer via -h switch

var (
	locationsGlobFlag = flag.String(
		"locations", "",
		"Path (glob) of locations csv file(s) to process")
	excludeFlag = flag.String(
		"exclude", "",
		"File to exclude (usually today's incomplete csv file)")
	allPathsFlag = flag.String(
		"all-paths", "",
		"Path of xml file with description of all paths to be processed")
	outputDirFlag = flag.String(
		"output", "",
		"Path of directory into which location-path csv files are to be written")
	overwriteFlag = flag.Bool(
		"overwrite", false,
		"Overwrite existing files if true, else append if false")
	parallelWorkersFlag = flag.Int(
		"parallel-workers", 0,
		"Number of worker go routines to start for processing CSV records "+
			"from a single file")
)

func main() {
	// Validate args.
	flag.Parse()
	ok := true

	// Are they set?
	if len(*locationsGlobFlag) == 0 {
		ok = false
		glog.Error("--locations not set")
	}
	if len(*allPathsFlag) == 0 {
		ok = false
		glog.Error("--all-paths not set")
	}
	if len(*outputDirFlag) == 0 {
		ok = false
		glog.Error("--output not set")
	}

	var matchingLocationFilePaths []string
	var err error

	if ok {
		// Are the values sensible?
		if !util.IsFile(*allPathsFlag) {
			ok = false
			glog.Errorf("Not a file: %v", *allPathsFlag)
		}
		if util.Exists(*outputDirFlag) && !util.IsDirectory(*outputDirFlag) {
			ok = false
			glog.Errorf("Not a directory: %v", *outputDirFlag)
		}
		matchingLocationFilePaths, err = filepath.Glob(*locationsGlobFlag)
		if err != nil {
			ok = false
			glog.Errorf("Error processing --locations flag: %v", err)
		} else if len(matchingLocationFilePaths) == 0 {
			ok = false
			glog.Error("--locations matched no files")
		}
	}

	if !ok {
		flag.PrintDefaults()
		return
	}

	util.InitGOMAXPROCS()

	glog.Infof("Reading paths from: %s", *allPathsFlag)
	agency, err := nextbus.ReadPathsFromFile(*allPathsFlag)
	if err != nil {
		glog.Fatal(err)
	} else if agency.NumPaths() == 0 {
		glog.Fatal("No paths in ", *allPathsFlag)
	}
	qtm := nbgeo.NewRouteToQuadTreeMap(agency)
	//	qt := nbgeo.NewQuadTreeWithAgencyPaths(agency)

	//	// Load quadtree with paths.  Would prefer to have min and max latitude
	//	// for the agency, but instead will narrow the the bounds after inserting
	//	// all the paths.
	//	glog.Infof("Add paths to quadtree")
	//	qt := geom.NewQuadTree(geom.NewRect(-180, +180, -90, 90))
	//	for _, paths := range agency.Paths {
	//		for _, path := range paths {
	//			nbgeo.AddPathToQuadTreeAsSegs(path, qt)
	//		}
	//	}
	//	qt.NarrowBounds()
	//	glog.Infof("Reduced quadtree bounds to: %v", qt.Bounds())

	if !util.Exists(*outputDirFlag) {
		err := os.MkdirAll(*outputDirFlag, 0755)
		if err != nil {
			glog.Fatal(err)
		}
	}

	outputChans := nbgeo.NewCsvOutputChanMap(
		*outputDirFlag, *overwriteFlag, 0644)
	defer func() {
		outputChans.CloseAll()
		glog.Info("Closed all output files")
	}()

	//	inputPathChan := make(chan[]string, 100)
	fileWG := &sync.WaitGroup{}

	// FOR NOW, processing one file at a time in order to simplify debugging.
	for _, filePath := range matchingLocationFilePaths {
		if filePath == *excludeFlag {
			continue
		}
		glog.Infof("Reading locations file:  %s", filePath)
		fileWG.Add(1)
		numRecords, err := nbgeo.LocationsFileToPerPathFiles(
			qtm, filePath, outputChans, fileWG, *parallelWorkersFlag)
		glog.Infof("Processed %6d locations from file: %s", numRecords, filePath)
		if err != nil {
			glog.Fatalf("Error while processing file: %s\n\tError: %v",
				filePath, err)
		}
	}

	glog.Info("Waiting for files to be processed")
	fileWG.Wait()
	glog.Info("Done waiting for files to be processed.")
}
