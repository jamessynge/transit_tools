package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	
	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
)

var (
	agencyFlag = flag.String(
		"agency", "",
		"Name of the transit agency.")
	destFlag = flag.String(
		"dest", "",
		"Directory into which to consolidate the fetched files from the roots")

	// Directories which may have tar.gz files in them (i.e. xml location responses).
	rawLocationsDirs []string 

	// Directories which may have csv.gz files in them (i.e. aggregated locations).
	csvLocationDirs []string 

	// Directories which may have routeList.xml files in them.
	configDirs []string 

	// Directories which may have routeList.xml files in them.
	logsDirs []string 
)

// Copy the file or directory 'from' to the new path 'to', preserving the dates.
// There must not already be a 'to' in existence.
func doCopy(from, to string) error {

	os.Chtimes(to, atime, mtime)
}

// Reads up to n entries from dir, unsorted.
func ReadDirN(dir string, n int) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(n)
	f.Close()
	if err != nil {
		return nil, err
	}
//	sort.Sort(byName(list))
	return list, nil
}







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








func checkOneDir(dir string) (hasConfig, hasRouteList, hasLocations, hasLogs, hasProcessed, hasRaw bool) {
	checkFiles := true
	if util.IsDirectory(filepath.Join(dir, "config")) {
		hasConfig = true
		checkFiles = false
	}
	if util.IsFile(filepath.Join(dir, "routeList.xml")) {
		hasRouteList = true
		checkFiles = false
	}

	if util.IsDirectory(filepath.Join(dir, "locations")) {
		hasLocations = true
		checkFiles = false
	}
	if util.IsDirectory(filepath.Join(dir, "logs")) {
		hasLogs = true
		checkFiles = false
	}
	if util.IsDirectory(filepath.Join(dir, "processed")) {
		hasProcessed = true
		checkFiles = false
	}
	if util.IsDirectory(filepath.Join(dir, "raw")) {
		hasRaw = true
		checkFiles = false
	}

	if checkFiles {
		if infos, err := ioutil.ReadDir(dir); err != nil {
			glog.Warningf("Unable to read from directory %s: %s", dir, err)
		} else {
			for info := range infos {
				if strings.HasSuffix(info.Name(), ".tar.gz") {
					hasRaw = true
					break
				}
			}
			for info := range infos {
				if strings.HasSuffix(info.Name(), ".csv.gz") {
					hasProcessed = true
					break
				}
			}
		}
	}
	return
}





// Dir (a path) is or has an ancestor with *agencyFlag in the name.
// Search within dir for fetched data. 
func searchAgencyDir(dir string) {
	// Does this directory contain any of the target directories?
	stop := false
	if util.IsDirectory(filepath.Join(dir, "config")) {
		configDirs = append(configDirs, filepath.Join(dir, "config"))
		stop = true
	}
	if util.IsDirectory(filepath.Join(dir, "config")) {
		configDirs = append(configDirs, filepath.Join(dir, "config"))
		stop = true
	}
	if 

	


}




func searchRoot(root string, insideAgencyDir bool) {
	if 


	// Does it contain the agency dir?
	if util.IsDirectory(filepath.Join(root, *agencyFlag)) {
		// Yup.  Search within only that dir.
		processRoot(filepath.Join(root, *agencyFlag), true)
		return
	}

	if strings.Contains(root, string(os.PathSeparator) + *agencyFlag) {
		// Already inside 







}



















func main() {
	flag.Parse()
	if len(*agencyFlag) == 0 { glog.Fatal("Need -agency"); }
	if len(*destFlag) == 0 { glog.Fatal("Need -dest"); }
	if !util.IsDirectory(*destFlag) { glog.Fatal("-dest must specify a directory"); }

	roots, err := util.ExpandPathGlobs(*rootsFlag, ",")




	err := nextbus.ParseRouteConfigsDir(agency, *routeConfigFlag)
	if err != nil {
		glog.Fatal(err)
	}
	fmt.Printf("Parsed %d routes, %d directions, %d stops, %d locations, %d paths\n",
		len(agency.Routes), len(agency.Directions), len(agency.Stops), len(agency.Locations), agency.NumPaths())
	nextbus.WritePathsToFile(agency, *allPathsFlag)
}
