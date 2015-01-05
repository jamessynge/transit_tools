// Partition vehicle reports by location (area). The partitions
// can either be determined from the data, or supplied as input
// (e.g. from a previous partitioning).
package main 

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/nextbus/nblocations"
	"github.com/jamessynge/transit_tools/util"
)

var (
	agencyFlag = flag.String(
		"agency", "",
		"Nextbus identifier for the transit agency (e.g. 'mbta' or 'ccrta').")

	locationsGlobFlag = flag.String(
		"location-roots", "",
		"Comma-separated list of paths (globs) to search for locations files " +
		"(e.g. .csv.gz files).")
	outputDirFlag = flag.String(
		"output", "",
		"Directory into which to write the geo-partitioned locations.")

	noPartitionFlag = flag.Bool(
		"no-partition", false,
		"Create index (if it doesn't exist), read the index, but don't partition " +
		"bulk data.")

	minLocationsFlag = flag.Int(
		"min-locations", 1000000,
		"Minimum number of locations to load in order to generate partitions.")

	partitionLevelsFlag = flag.Uint(
		"partition-levels", 6,
		"Number of levels of partitioning (alternating between east-west and " +
		"north-south partitioning); multiple of two recommended; first two " +
		"levels are special, always having two leaf partitions and a central " +
		"partition that contains essentially all of the data")
	cutsPerLevelFlag = flag.Uint(
		"cuts-per-level", 2,
		"At each level, how many cuts should be made; at least 1.")

	setLogDirFlag = flag.Bool(
		"set_log_dir", true,
		"Set glog's --log_dir default value immediately after parsing flags so that "+
			"messages are logged there immediately, and not after the file is rotated.")
)

func checkFlags() {
	if len(*agencyFlag) == 0 {
		glog.Fatal("Must specify --agency")
	}
	if len(*locationsGlobFlag) == 0 {
		glog.Fatal("Must specify --location-roots")
	}
	if len(*outputDirFlag) == 0 {
		glog.Fatal("Must specify --output")
	}
	if util.Exists(*outputDirFlag) && !util.IsDirectory(*outputDirFlag) {
		glog.Fatalf("--output=%q does not specify a directory", *outputDirFlag)
	}
	if *partitionLevelsFlag < 1 {
		glog.Fatalf("--partition-levels=%d is too low", *partitionLevelsFlag)
	}
	if *cutsPerLevelFlag < 1 {
		glog.Fatalf("--cuts-per-level=%d is too low", *cutsPerLevelFlag)
	}

	// Set --log_dir before any logging with glog (except the Fatal calls above)
	// so that the log files are in the correct location.
	if *setLogDirFlag {
		util.SetDefaultLogDir(filepath.Join(*outputDirFlag, "logs"))
	}
}

////////////////////////////////////////////////////////////////////////////////

func FindAllFiles(globs string) (filePaths []string) {
	pat := string(os.PathSeparator) + *agencyFlag + string(os.PathSeparator)
	nblocations.FindCsvLocationsFiles(globs, func(filePath string) bool {
		if strings.Contains(filePath, pat) {
			filePaths = append(filePaths, filePath)
		} else {
			glog.V(1).Infof("Ignoring non-agency path: %s", filePath)
		}
		return true
	})
	return
}

// Sort files in ascending time order.
func SortFiles(filePaths []string, reverse bool) {
	re := regexp.MustCompile(`^(\d\d\d\d)[-_]?(\d\d)[-_](\d\d)[-_.]`)
	dates := make(map[string]time.Time)
	for _, filePath := range filePaths {
		m := re.FindStringSubmatch(filepath.Base(filePath))
		var t time.Time
		if len(m) >= 4 {
			t, _ = time.Parse("20060102", m[1] + m[2] + m[3])
		} else {
			if stat, err := os.Stat(filePath); err == nil {
				t = stat.ModTime()  // Is there a way to get ctime?
			} else {
				glog.Warningf("Unable to determine time for: %s\nError: %s", filePath, err)
			}
		}
		dates[filePath] = t
	}
	less := func(i, j int) bool {
		if reverse {
			i, j = j, i
		}
		a := dates[filePaths[i]]
		b := dates[filePaths[j]]
		if a.Before(b) {
			return true
		} else if b.Before(a) {
			return false
		} else {
			return filePaths[i] < filePaths[j]
		}
	}
	swap := func(i, j int) {
		filePaths[i], filePaths[j] = filePaths[j], filePaths[i]
	}
	util.Sort3(len(filePaths), less, swap)
}

func LoadSomeLocations(
		sortedFilePaths []string, atLeast int) (locations []geo.Location) {
	for _, f := range sortedFilePaths {
		rals, err := nblocations.LoadRecordsAndLocation(f)
		if err != nil {
			glog.Warningf("Error reading records after %d from %s\nError: %s",
										len(rals), f, err)
		}
		if len(rals) > 0 {
			l := make([]geo.Location, 0, len(rals))
			for _, ral := range rals {
				l = append(l, ral.Location)
			}
			locations = append(locations, l...)
			if len(locations) >= atLeast {
				glog.Infof("Accumulated %d locations", len(locations))
				return
			}
			glog.Infof("Accumulated %d locations...", len(locations))
		}
	}
	glog.Warningf(
		"After reading from %d files, only found %d locations, not at least %d",
		len(sortedFilePaths), len(locations), atLeast)
	return
}

func CreateAgencyPartitioner(filePaths []string) *nblocations.AgencyPartitioner {
	// Use the most recent files for this task.
	SortFiles(filePaths, true)
	locations := LoadSomeLocations(filePaths, *minLocationsFlag)

	return nblocations.CreateAgencyPartitioner(
			*agencyFlag, locations, *partitionLevelsFlag, *cutsPerLevelFlag)
}

func ReadOrCreatePartitioner() *nblocations.AgencyPartitioner {
	// To ensure we always operate the same way, we create and save the
	// partitions index, then read it back from the file. 
	if !util.IsFile(nblocations.GetPartitionsIndexPath(*outputDirFlag)) {
		filePaths := FindAllFiles(*locationsGlobFlag)
		a := CreateAgencyPartitioner(filePaths)
		nblocations.SavePartitionsIndex(*outputDirFlag, a)
	}
	return nblocations.ReadPartitionsIndex(*outputDirFlag)
}

////////////////////////////////////////////////////////////////////////////////

type CRC struct {
	filePath string
	crc *util.CsvReaderCloser
}

func OpenCsvFilesSendToCh(
		filePaths []string, ch chan<- CRC,
		wg *sync.WaitGroup) {
	for _, filePath := range filePaths {
		crc, err := util.OpenReadCsvFile(filePath)
		if err != nil {
			glog.Warningf("Unable to open %s:\nError: %s", filePath, err)
			continue
		}
		ch <- CRC{filePath: filePath, crc: crc}
	}
	close(ch)
	wg.Done()
}

func ReadCRCs(partitioner nblocations.Partitioner,
							ch <-chan CRC, wg *sync.WaitGroup) {
	totalRecords := 0
	for {
		crc, ok := <-ch
		if !ok { break }
		glog.Infof("Reading from %s", crc.filePath)
		records := 0
		for {
			var e error
			if record, e1 := crc.crc.Read(); e1 == nil {
				records++
				if ral, e2 := nblocations.CreateRecordAndLocation(record); e2 == nil {
					e3 := partitioner.Partition(ral)
					if e3 == nil {
						continue
					}
					e = e3
				} else {
					e = e2
				}
			} else {
				e = e1
			}
			if e == io.EOF {
				crc.crc.Close()
				glog.Infof("Done reading %d records from from %s", records, crc.filePath)
				break
			}
			if e != nil {
				glog.Warningf("Error reading from %s: %s", crc.filePath, e)
				break
			}
		}
		totalRecords += records
	}
	glog.Infof("Done reading %d records from CSV files", totalRecords)
	wg.Done()
}

////////////////////////////////////////////////////////////////////////////////

func main() {
	flag.Parse()
	checkFlags()
	defer glog.Flush()	// Flush the files when shutting down
	util.InitGOMAXPROCS()
	partitioner := ReadOrCreatePartitioner()
	if *noPartitionFlag {
		return
	}

	partitioner.OpenForWriting(*outputDirFlag, true)

	filePaths := FindAllFiles(*locationsGlobFlag)
	SortFiles(filePaths, false)

	ch := make(chan CRC)  // Unbuffered, so we'll only open one file ahead of
												// the 
	var wg sync.WaitGroup

	wg.Add(1)
	go ReadCRCs(partitioner, ch, &wg)

	glog.Infof("Start to read %d files", len(filePaths))
	wg.Add(1)
	go OpenCsvFilesSendToCh(filePaths, ch, &wg)

	wg.Wait()
	glog.Infof("Done reading %d files", len(filePaths))

	partitioner.Close()
	glog.Info("Closed partition files")
	
	
//	nblocations.FindCsvLocationsFiles
}
