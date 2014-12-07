package main 

import (
	"flag"
"path/filepath"
"strings"
"os"
"time"

"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/nextbus/configfetch"

"github.com/jamessynge/transit_tools/util"
)

var execFlag = flag.Bool(
	"exec", false,
	"If true, cleans config directories when those on either side in time " +
	"order are the same; else just logs the info.")

// Find directories under root that contain "routeList.xml".
func findConfigDirs(root string, timestamps map[string]time.Time) ([]string,  error) {
	dirsToSkip := map[string]bool{
		"locations": true,
		"logs": true,
		"raw": true,
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
		dirs []string, timestamps map[string]time.Time) ([]string,  error) {
	var allConfigDirs []string
	for _, root := range dirs {
		matches, err := filepath.Glob(root)
		if err != nil {
			glog.Errorln("Error expanding glob", root, "\nError:", err)
			return nil, err
		}
		var configDirs []string
		if len(matches) != 1 || matches[0] != root {
			glog.Infoln("Expanded glob", root, "to", len(matches),"matches")
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

	// Want to keep the first and last of the sequence that are identical,
	// not just the first or last.
	firstDir := ""
	prevDir := ""
	var candidates []string
	for _, currentDir := range allConfigDirs {
		if prevDir == "" {
			glog.V(1).Info("prevDir is empty")
			firstDir = currentDir
			prevDir = currentDir
			continue
		}
		if !CompareConfigDirs(prevDir, currentDir) {
			glog.V(1).Info("Resetting firstDir and prevDir to currentDir")
			firstDir = currentDir
			prevDir = currentDir
			continue
		}
		if firstDir == prevDir {
			glog.V(1).Info("firstDir and prevDir are the same, no middle dir yet")
			prevDir = currentDir
			continue
		}
		// We've got three in a row that are the same.
		glog.Infof(`Found 3 config dirs in a row that are the same:
 First: %s
Second: %s
 Third: %s`, firstDir, prevDir, currentDir)
 		candidates = append(candidates, prevDir)
		if *execFlag {
			glog.Infoln("Removing middle dir:", prevDir)
			err := os.RemoveAll(prevDir)
			if err != nil {
				glog.Fatalf("Failed to remove %s\nError: %s", prevDir, err)
				return
			}
		} else {
			glog.V(1).Infoln("Added to candidates:", prevDir)
		}
		prevDir = currentDir
	}

	if *execFlag {
		glog.Infoln("Removed", len(candidates), "duplicate config dir(s)")
	} else {
		glog.Infoln("There are", len(candidates), "directories to remove:\n",
				strings.Join(candidates, "\n"))
	}
}
