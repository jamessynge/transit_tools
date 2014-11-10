// go run check_csv_order.go C:/temp/fetch_vehicles5/csv  C:/temp/fetch_vehicles5b/csv  C:/temp/fetch_vehicles5c  C:/temp/fetch_vehicles5d  C:/temp/fetch_vehicles5e
package main 

import (
"flag"
"github.com/golang/glog"
"io"
"io/ioutil"
"os"
"path/filepath"
"strconv"
"strings"
"util"
)

func ProcessCsvFile(path string) {
	crc, err := util.OpenReadCsvFile(path)
	if err != nil {
		glog.Infof("Unable to open CSV file %s\nError: %s", path, err)
		return
	}
	defer crc.Close()
	numRecords := 0
	fileOk := true
	var lastTime int64 = 946684800000  // 2000-01-01 @ 0:0:0.000 UTC
	var lastRecord []string
	for {
		thisRecord, err := crc.Read()
		if err != nil {
			if err != io.EOF {
				glog.Errorf("CSV read error: %s\nFile: %s", err, path)
				fileOk = false
			}
			break
		}
		numRecords++
		thisTime, err := strconv.ParseInt(thisRecord[0], 10, 64)
		if err != nil {
			glog.Errorf("Unable to parse time in record %d of file %s", numRecords, path)
			glog.Errorf("thisRecord: %v", thisRecord)
			fileOk = false
			continue
		}
		if thisTime < lastTime {
			glog.Warningf("Time moving backwards at record %d of file %s", numRecords, path)
			glog.Warningf("lastRecord: %v", lastRecord)
			glog.Warningf("thisRecord: %v", thisRecord)
			glog.Warningf("  delta Ms: %d", thisTime - lastTime)
			fileOk = false
			continue
		}
		lastTime = thisTime
		lastRecord = thisRecord
	}
	if fileOk {
		glog.Infof("File of %d records is correct: %s", numRecords, path)
	} else {
		glog.Infof("File of %d records completed: %s", numRecords, path)
	}
}

func ProcessFile(path string) {
	if strings.HasSuffix(path, ".csv") {
		ProcessCsvFile(path)
	} else if strings.HasSuffix(path, ".csv.gz") {
		ProcessCsvFile(path)
	} else {
		glog.Infof("Ignoring file: %s", path)
	}
}

func ProcessDir(path string) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		glog.Infof("Unable to read directory %s\nError: %s\n", path, err)
		return
	}
	glog.Infof("Enter: %s", path)
	for _, elem := range files {
		Process(filepath.Join(path, elem.Name()))
	}
	glog.Infof(" Exit: %s", path)
}

func Process(path string) {
	if util.IsFile(path) {
		ProcessFile(path)
	} else if util.IsDirectory(path) {
		ProcessDir(path)
	} else {
		glog.Infof("Not a file, nor a directory: %s", path)
	}
}

func main() {
	flag.Lookup("alsologtostderr").Value.Set("true")
	flag.Parse()

	for _, arg := range os.Args[1:] {
		matches, _ := filepath.Glob(arg)
		if len(matches) > 0 {
//			glog.Infof("matches: %v", matches)
			for _, match := range matches {
//				glog.Infof("%d: %s", ndx, match)
				Process(match)
			}
		} else {
			Process(arg)
		}
	}
}
