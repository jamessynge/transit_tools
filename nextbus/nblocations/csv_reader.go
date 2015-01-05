package nblocations

// Find and read a nnn.csv.gz files, producing []string slices.

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
)

func shouldSkipDir(root string) bool {
	b := filepath.Base(root)
	return b == "config" || b == "logs"
}

type ReceivePathFn func(path string) (keepGoing bool)

func FindCsvLocationsFilesUnderRoot(
	root string, fn ReceivePathFn) (stop bool) {
	walkFn := func(fp string, info os.FileInfo, err error) error {
		if stop {
			return io.EOF
		}
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipDir(fp) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(fp, ".csv.gz") || strings.HasSuffix(fp, ".csv") {
			if !fn(fp) {
				stop = true
				return io.EOF
			}
		}
		return nil
	}
	glog.Infoln("Searching under root", root)
	filepath.Walk(root, walkFn)
	return
}

func FindCsvLocationsFiles(rootGlobs string, fn ReceivePathFn) {
	roots, err := util.ExpandPathGlobs(rootGlobs, ",")
	if err != nil {
		glog.Errorln(err)
	}
	for _, root := range roots {
		if FindCsvLocationsFilesUnderRoot(root, fn) {
			return
		}
	}
}

func CreateRecordAndLocation(record []string) (*RecordAndLocation, error) {
	rl := &RecordAndLocation{Record: record}
	err := nextbus.CSVFieldsIntoVehicleLocation(record, &rl.VehicleLocation)
	return rl, err
}

func LoadRecordsAndLocation(filePath string) (
	s []*RecordAndLocation, err error) {
	fn := func(
		source string, record []string, recordNum int, err error) error {
		if err != nil {
			return err
		}
		if len(record) > 0 {
			rl, err := CreateRecordAndLocation(record)
			if rl != nil {
				s = append(s, rl)
			}
			return err
		}
		return nil
	}
	glog.Infof("Reading RecordAndLocation from: %s", filePath)
	_, err = util.ReadCsvFileToFn(filePath, fn)
	return
}
