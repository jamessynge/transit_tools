package main

import (
	"flag"
	"log"
	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/nextbus/nbgeo"
	"os"
	"path/filepath"
	"github.com/jamessynge/transit_tools/util"
	//   "runtime"
	//	"github.com/jamessynge/transit_tools/geom"
	"sync"
)

// The flag package provides a default help printer via -h switch

var (
	locationsGlobFlag = flag.String(
		"locations", "",
		"Path (glob) of locations csv file(s) to process")
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
		log.Print("--locations not set")
	}
	if len(*allPathsFlag) == 0 {
		ok = false
		log.Print("--all-paths not set")
	}
	if len(*outputDirFlag) == 0 {
		ok = false
		log.Print("--output not set")
	}

	var matchingLocationFilePaths []string
	var err error

	if ok {
		// Are the values sensible?
		if !util.IsFile(*allPathsFlag) {
			ok = false
			log.Printf("Not a file: %v", *allPathsFlag)
		}
		if util.Exists(*outputDirFlag) && !util.IsDirectory(*outputDirFlag) {
			ok = false
			log.Printf("Not a directory: %v", *outputDirFlag)
		}
		matchingLocationFilePaths, err = filepath.Glob(*locationsGlobFlag)
		if err != nil {
			ok = false
			log.Printf("Error processing --locations flag: %v", err)
		} else if len(matchingLocationFilePaths) == 0 {
			ok = false
			log.Print("--locations matched no files")
		}
	}

	if !ok {
		flag.PrintDefaults()
		return
	}

	util.InitGOMAXPROCS()

	log.Printf("Reading paths from: %s", *allPathsFlag)
	agency, err := nextbus.ReadPathsFromFile(*allPathsFlag)
	if err != nil {
		log.Panic(err)
	} else if agency.NumPaths() == 0 {
		log.Panic("No paths in ", *allPathsFlag)
	}
	qtm := nbgeo.NewRouteToQuadTreeMap(agency)
	//	qt := nbgeo.NewQuadTreeWithAgencyPaths(agency)

	//	// Load quadtree with paths.  Would prefer to have min and max latitude
	//	// for the agency, but instead will narrow the the bounds after inserting
	//	// all the paths.
	//	log.Printf("Add paths to quadtree")
	//	qt := geom.NewQuadTree(geom.NewRect(-180, +180, -90, 90))
	//	for _, paths := range agency.Paths {
	//		for _, path := range paths {
	//			nbgeo.AddPathToQuadTreeAsSegs(path, qt)
	//		}
	//	}
	//	qt.NarrowBounds()
	//	log.Printf("Reduced quadtree bounds to: %v", qt.Bounds())

	if !util.Exists(*outputDirFlag) {
		err := os.MkdirAll(*outputDirFlag, 0755)
		if err != nil {
			log.Panic(err)
		}
	}

	outputChans := nbgeo.NewCsvOutputChanMap(
		*outputDirFlag, *overwriteFlag, 0644)
	defer func() {
		outputChans.CloseAll()
		log.Printf("Closed all output files")
	}()

	//	inputPathChan := make(chan[]string, 100)
	fileWG := &sync.WaitGroup{}

	// FOR NOW, processing one file at a time in order to simplify debugging.
	fileWG.Add(len(matchingLocationFilePaths))
	for _, filePath := range matchingLocationFilePaths {
		log.Printf("Reading locations file:  %s", filePath)
		numRecords, err := nbgeo.LocationsFileToPerPathFiles(
			qtm, filePath, outputChans, fileWG, *parallelWorkersFlag)
		log.Printf("Processed %6d locations from file: %s", numRecords, filePath)
		if err != nil {
			log.Panicf("Error while processing file: %s\n\tError: %v",
				filePath, err)
		}
	}

	//TODO	log.Printf("
	fileWG.Wait()
	log.Printf("Done waiting for files to be processed.")

	//	log.Printf("
	/*
		for i, limit := 0, runtime.NumCPU()-1; i < limit; i++ {
			go func() {
				for {
					filePath, ok := <-inputPathChan
					if !ok { return }

					nbgeo.LocationsFileToPerPathFiles(
							qt, filePath, outputChans,


				}
			}()
		}*/
}
