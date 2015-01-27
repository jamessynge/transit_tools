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
	"runtime"
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
		"Comma-separated list of paths (globs) to search for locations files "+
			"(e.g. .csv.gz files).")
	outputDirFlag = flag.String(
		"output", "",
		"Directory into which to write the geo-partitioned locations.")

	createIndexFlag = flag.Bool(
		"create-index", false,
		"Create index, overwriting existing index if present.")
	minLocationsFlag = flag.Int(
		"min-locations", 1000000,
		"Minimum number of locations to load in order to generate partitions.")
	squarePartitionsFlag = flag.Bool(
		"square-partition", true,
		"Create index (if it doesn't exist), read the index, but don't partition "+
			"bulk data.")

	// Non-square partitions flags:
	partitionLevelsFlag = flag.Uint(
		"partition-levels", 6,
		"Number of levels of partitioning (alternating between east-west and "+
			"north-south partitioning); multiple of two recommended; first two "+
			"levels are special, always having two leaf partitions and a central "+
			"partition that contains essentially all of the data")
	cutsPerLevelFlag = flag.Uint(
		"cuts-per-level", 2,
		"At each level, how many cuts should be made; at least 1.")

	// Square partitions flags:
	minSquareSideFlag = flag.Uint(
		"min-square-side", 768,
		"Minimum number of meters on a square partition side.")
	maxSquareSideFlag = flag.Uint(
		"max-square-side", 8192,
		"Maximum number of meters on a square partition side.")
	maxSamplesFractionFlag = flag.Float64(
		"max-samples-fraction", 0.005,
		"Maximum fraction of the samples used for creating the index that may " +
		"be in one partition's area, unless the partition would be too small.")

	noPartitionFlag = flag.Bool(
		"no-partition", false,
		"Create index (if it doesn't exist), read the index, but don't partition "+
			"bulk data.")

	setLogDirFlag = flag.Bool(
		"set-log_dir", true,
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

	if *minSquareSideFlag < 1 {
		glog.Fatalf("--min-square-side=%d is too low", *minSquareSideFlag)
	}
	if *maxSquareSideFlag <= *minSquareSideFlag {
		glog.Fatalf("--max-square-side=%d is too low", *maxSquareSideFlag)
	}

	if 0.1 < *maxSamplesFractionFlag {
		glog.Fatalf("--max-samples-fraction=%v is too high", *maxSamplesFractionFlag)
	}
	if *maxSamplesFractionFlag <= 0 {
		glog.Fatalf("--max-samples-fraction=%v is too low", *maxSamplesFractionFlag)
	}
	maxSamples := int(*maxSamplesFractionFlag * float64(*minLocationsFlag))
	if maxSamples <= 0 {
		glog.Fatalf("--max-samples-fraction=%v is too low", *maxSamplesFractionFlag)
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
			t, _ = time.Parse("20060102", m[1]+m[2]+m[3])
		} else {
			if stat, err := os.Stat(filePath); err == nil {
				t = stat.ModTime() // Is there a way to get ctime?
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

	if *squarePartitionsFlag {
		return nblocations.CreateAgencyPartitioner3(
			*agencyFlag, locations,
			geo.Meters(*minSquareSideFlag), geo.Meters(*maxSquareSideFlag),
			*maxSamplesFractionFlag)
	} else {
		return nblocations.CreateAgencyPartitioner(
			*agencyFlag, locations, *partitionLevelsFlag, *cutsPerLevelFlag)
	}
}

func ReadOrCreatePartitioner() *nblocations.AgencyPartitioner {
	// To ensure we always operate the same way, we create and save the
	// partitions index, then read it back from the file.
	if *createIndexFlag || !util.IsFile(nblocations.GetPartitionsIndexPath(*outputDirFlag)) {
		filePaths := FindAllFiles(*locationsGlobFlag)
		a := CreateAgencyPartitioner(filePaths)
		nblocations.SavePartitionsIndex(*outputDirFlag, a)
	}
	return nblocations.ReadPartitionsIndex(*outputDirFlag)
}

////////////////////////////////////////////////////////////////////////////////

func ReadAnOpenedCsvInputFile(
		partitioner nblocations.Partitioner,
		filePath string, crc *util.CsvReaderCloser) int64 {
	var wg sync.WaitGroup
	csvRecordsCh := make(chan []string, 100)

	// Function that will be used for N go routines, so that we can make use of
	// multiple processors.
	recordGR := func() {
		defer wg.Done()
		for {
			record, ok := <-csvRecordsCh
			if !ok { return }
			if ral, e := nblocations.CreateRecordAndLocation(record); e != nil {
				glog.Warningf("Error parsing record: %v\nRecord: %v", e, record)
				continue
			} else if e = partitioner.Partition(ral); e != nil {
				glog.Warningf("Error partitioning RecordAndLocation: %v\nRecord: %v\nRAL: %v", e, record, ral)
				continue
			}
		}
	}
	cpus := runtime.NumCPU()
	for n := 0; n < cpus; n++ {
		wg.Add(1)
		go recordGR()
	}
	glog.Infof("Reading from %s", filePath)
	records := int64(0)
	errorCount := 0
	for {
		if record, e := crc.Read(); e == nil {
			records++
			csvRecordsCh <- record
			errorCount = 0
		} else if e == io.EOF {
			glog.Infof("Done reading %d records from from %s", records, filePath)
			break
		} else {
			glog.Warningf("Error reading from %s: %s", filePath, e)
			errorCount++
			if errorCount > 10 {
				glog.Warningf("Giving up on reading from %s after %d records", filePath, records)
				break
			}
		}
	}

	close(csvRecordsCh)
	go crc.Close()
	wg.Wait()
	
	return records
}

////////////////////////////////////////////////////////////////////////////////

type OpenedCsvInputFile struct {
	filePath string
	crc      *util.CsvReaderCloser
}

func OpenCsvFilesSendToCh(
	filePaths []string, ch chan<- OpenedCsvInputFile, wg *sync.WaitGroup) {
	defer wg.Done()
	for _, filePath := range filePaths {
		crc, err := util.OpenReadCsvFileAndPump(filePath)
		if err != nil {
			glog.Warningf("Unable to open %s:\nError: %s", filePath, err)
			continue
		}
		crc.Comment = '#'
		ch <- OpenedCsvInputFile{filePath: filePath, crc: crc}
	}
	close(ch)
}

func ReadOpenedCsvInputFiles(partitioner nblocations.Partitioner,
	ch <-chan OpenedCsvInputFile, wg *sync.WaitGroup) {
	defer wg.Done()
	totalRecords := int64(0)
	for {
		cif, ok := <-ch
		if !ok {
			break
		}
		records := ReadAnOpenedCsvInputFile(partitioner, cif.filePath, cif.crc)
		totalRecords += records
	}
	glog.Infof("Done reading %d records from CSV files", totalRecords)
}

////////////////////////////////////////////////////////////////////////////////

func main() {
	flag.Parse()
	checkFlags()
	defer glog.Flush() // Flush the files when shutting down
	util.InitGOMAXPROCS()
	partitioner := ReadOrCreatePartitioner()
	if *noPartitionFlag {
		return
	}

	partitioner.OpenForWriting(*outputDirFlag, true)

	filePaths := FindAllFiles(*locationsGlobFlag)
	SortFiles(filePaths, false)

	// Unbuffered, so we'll only open one file ahead of
	// the one that we're processing.
	ch := make(chan OpenedCsvInputFile)
	var wg sync.WaitGroup

	wg.Add(1)
	go ReadOpenedCsvInputFiles(partitioner, ch, &wg)

	glog.Infof("Start to read %d files", len(filePaths))
	wg.Add(1)
	go OpenCsvFilesSendToCh(filePaths, ch, &wg)

	wg.Wait()
	glog.Infof("Done reading %d files", len(filePaths))

	partitioner.Close()
	glog.Info("Closed partition files")

	//	nblocations.FindCsvLocationsFiles
}
