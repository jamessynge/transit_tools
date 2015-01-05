package main

// TODO Use configfetch.NewConfigCuller instead of reimplementing it here
// (well, this was the original implementation, but NewConfigCuller was based
// on this one).

// TODO Accomodate configfetch failures by detecting when the differences
// between two config dirs is due to a failure (e.g. one only collected a
// small subset of the files, or one is a superset of the other, i.e. the
// list of all routes is the same, but more routes/schedules were fetched by
// one than by the other).

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/nextbus/configfetch"

	"github.com/jamessynge/transit_tools/util"
)

var execFlag = flag.Bool(
	"exec", false,
	"If true, cleans config directories when those on either side in time "+
		"order are the same; else just logs the info.")

// Find directories under root that contain "routeList.xml".
func findConfigDirs(root string, timestamps map[string]time.Time) ([]string, error) {
	dirsToSkip := map[string]bool{
		"locations": true,
		"logs":      true,
		"raw":       true,
		"processed": true,
	}
	errs := util.NewErrors()
	configDirs := []string{}
	walkFn := func(fp string, info os.FileInfo, err error) error {
		if err != nil {
			errs.AddError(err)
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		bn := filepath.Base(fp)
		if dirsToSkip[bn] {
			return filepath.SkipDir
		}
		fp2 := filepath.Join(fp, "routeList.xml")
		if util.IsFile(fp2) {
			configDirs = append(configDirs, fp)
			// "Ideally", might get timestamp from inside routeList.xml file, but
			// perhaps a bit excessive.
			timestamps[fp] = info.ModTime()
			return filepath.SkipDir
		}
		return nil
	}
	glog.Infoln("Searching under root", root)
	err := filepath.Walk(root, walkFn)
	if err != nil {
		return nil, err
	}
	return configDirs, errs.ToError()
}

func findAllConfigDirs(
	dirs []string, timestamps map[string]time.Time) ([]string, error) {
	var allConfigDirs []string
	for _, root := range dirs {
		matches, err := filepath.Glob(root)
		if err != nil {
			glog.Errorln("Error expanding glob", root, "\nError:", err)
			return nil, err
		}
		var configDirs []string
		if len(matches) != 1 || matches[0] != root {
			glog.Infoln("Expanded glob", root, "to", len(matches), "matches")
			configDirs, err = findAllConfigDirs(matches, timestamps)
			glog.Infoln("Finished with glob", root)
		} else {
			configDirs, err = findConfigDirs(root, timestamps)
		}
		if err != nil {
			glog.Errorln("Error searching for config dirs under", root, "\nError:", err)
			return nil, err
		}
		allConfigDirs = append(allConfigDirs, configDirs...)
	}
	return allConfigDirs, nil
}

/*
func CompareConfigDirs(dir1, dir2 string) bool {
	glog.Infoln("Comparing:", dir1)
	glog.Infoln("  Against:", dir2)
	eq, err := configfetch.CompareConfigDirs(dir1, dir2)
	if eq && err == nil {
		glog.Infoln("Configurations are the same")
		return true
	} else if err != nil {
		glog.Infoln("Errors while comparing directories:\n", err)
	} else {
		glog.Infoln("Configurations are different")
	}
	return false
}
*/
func main() {
	flag.Parse()
	timestamps := make(map[string]time.Time)
	allConfigDirs, err := findAllConfigDirs(flag.Args(), timestamps)
	if err != nil {
		return
	}

	// Sort the config dirs by modified date.
	less := func(i, j int) bool {
		timeI := timestamps[allConfigDirs[i]]
		timeJ := timestamps[allConfigDirs[j]]
		return timeI.Before(timeJ)
	}
	swap := func(i, j int) {
		allConfigDirs[j], allConfigDirs[i] = allConfigDirs[i], allConfigDirs[j]
	}
	util.Sort3(len(allConfigDirs), less, swap)

	// Send the config dirs to the culler.
	candidatesCh := make(chan string, len(allConfigDirs))
	culler := configfetch.NewConfigCuller2(!*execFlag, candidatesCh)
	for _, currentDir := range allConfigDirs {
		culler.AddDir(currentDir)
	}

	// Wait for the culler to finish (depending here on the fact that the
	// culler has a zero length channel for receiving dirs).
	stoppedCh := culler.StopWhenReady()
	<-stoppedCh

	// Read the list of directories to remove/that were removed.
	var candidates []string
	for dir := range candidatesCh {
		candidates = append(candidates, dir)
	}

	if *execFlag {
		glog.Infoln("Removed", len(candidates), "duplicate config dir(s)")
	} else {
		glog.Infoln("There are", len(candidates), "directories to remove:\n",
			strings.Join(candidates, "\n"))
	}
}
