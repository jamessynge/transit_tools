// Read all the batched location reports (one tar.gz file per day, containing
// xml files), and produce a gzipped csv output file for each day, in a
// directory for year and month (e.g. 2013/07/01.reports.csv.gz).
// The CSV format is:
//
//   timestamp, date time, vehicle id, route tag, direction tag, heading, latitude, longitude
//
// TODO Emit the strings for the latitude and longitude that we originally
// read, not after converting to float64 and back to a string.

package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"github.com/jamessynge/transit_tools/nextbus"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var from_dir = flag.String(
	"from", "C:\\temp\\raw-mbta-locations",
	"Directory of gzipped tar files with nextbus location reports.")
var to_dir = flag.String(
	"to", "C:\\mbta\\daily-locations",
	"Directory into which to write gzipped csv files, one per day.")

func Exists(name string) bool {
	fi, err := os.Stat(name)
	//	log.Printf("Exists: err=%v     fi=%v", err, fi)
	return err == nil && fi != nil
}

type ParsedVehicleLocations struct {
	Locations []*nextbus.VehicleLocation
	Err       error
}

func ReadArchive(archivePath string, c chan<- ParsedVehicleLocations) error {
	// Open the tar archive for reading.
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	numLocations := 0
	defer func() {
		f.Close()
		log.Printf("Done reading %d locations from: %v", numLocations, archivePath)
	}()
	r, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	//	r := bufio.NewReader(f)
	tr := tar.NewReader(r)
	log.Printf("Reading from: %v", archivePath)
	for {
		var result ParsedVehicleLocations
		header, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		} else if err != nil {
			result.Err = err
			log.Printf("ERROR reading archive entry header: %v", result.Err)
		} else if header == nil {
			result.Err = fmt.Errorf("tar header missing from archive %v", archivePath)
			log.Printf("ERROR reading archive entry header: %v", result.Err)
		} else {
			//			log.Printf("Found archive entry: %v", header.Name)
			result.Locations, result.Err = nextbus.ReadXmlVehicleLocations(tr)
			numLocations += len(result.Locations)
		}
		c <- result
		//		log.Printf("Wrote to c1: %v", result)
	}
	return nil
}

type pendingVechicleLocation struct {
	firstReport         *nextbus.VehicleLocation
	numReports          int
	sumUnixMilliseconds int64
}

func (p *pendingVechicleLocation) IsSame(report *nextbus.VehicleLocation) bool {
	return p.firstReport != nil && p.firstReport.IsSameReport(report)
}

func (p *pendingVechicleLocation) ProduceOutput() (priorLocation *nextbus.VehicleLocation) {
	if p.numReports > 0 {
		if p.numReports > 1 {
			floatAvgMillis := float64(p.sumUnixMilliseconds) / float64(p.numReports)
			avgUnixMilliseconds := int64(math.Floor(floatAvgMillis + 0.5))
			t := time.Unix(avgUnixMilliseconds/1000,
				(avgUnixMilliseconds%1000)*1000000)

			//			log.Printf("Merged %d reports for vehicle %v to produce time: %v",
			//							   p.numReports, p.firstReport.VehicleId, t)

			priorLocation = new(nextbus.VehicleLocation)
			*priorLocation = *p.firstReport
			priorLocation.Time = t
		} else {
			priorLocation = p.firstReport
		}
	}
	p.firstReport = nil
	p.numReports = 0
	p.sumUnixMilliseconds = 0
	return
}

func (p *pendingVechicleLocation) Send(out chan<- *nextbus.VehicleLocation) {
	if o := p.ProduceOutput(); o != nil {
		//		log.Printf("Sending report for vehicle: %v", o.VehicleId)
		out <- o
	}
}

func (p *pendingVechicleLocation) MergeReports(report *nextbus.VehicleLocation,
	out chan<- *nextbus.VehicleLocation) {
	if p.numReports > 0 {
		if p.IsSame(report) {
			p.numReports++
			p.sumUnixMilliseconds += report.UnixMilliseconds()
			//			log.Printf("Merging reports %d for vehicle: %v", p.numReports, report.VehicleId)
			return
		}
		p.Send(out)
	}
	p.firstReport = report
	p.numReports = 1
	p.sumUnixMilliseconds = report.UnixMilliseconds()
	return
}

func SendUnseen(pending map[string]*pendingVechicleLocation,
	seen map[string]bool,
	out chan<- *nextbus.VehicleLocation) {
	//	log.Printf("SendUnseen of %d vehicles, %d in last report", len(pending), len(seen))
	for id := range pending {
		if seen[id] {
			continue
		}
		pending[id].Send(out)
		delete(pending, id)
	}
}

func ProcessParsedVehicleLocations(in <-chan ParsedVehicleLocations,
	out chan<- *nextbus.VehicleLocation) {
	defer close(out)
	// Yes, this is ridiculous overkill: it produces sub-second accuracy on
	// the time of a vehicle location report by averaging the times from
	// several report messages.
	pending := make(map[string]*pendingVechicleLocation)
	for {
		reports, ok := <-in
		// Has channel been closed?
		if !ok {
			break // Yup
		}
		// Ignore read errors, and empty reports (such as in the middle of the night).
		if reports.Err != nil {
			log.Printf("Error parsing report: %v", reports.Err)
			continue
		}
		if len(reports.Locations) == 0 {
			continue
		}
		// We have some vehicle location reports.
		seen := make(map[string]bool)
		for _, v := range reports.Locations {
			id := v.VehicleId
			seen[id] = true
			p, ok := pending[id]
			if !ok {
				p = new(pendingVechicleLocation)
				pending[id] = p
			}
			p.MergeReports(v, out)
		}
		SendUnseen(pending, seen, out)
	}
	SendUnseen(pending, nil, out)
}

func SortAndBatchVehicleLocations(
	pendingLimit, batchSize int,
	in <-chan *nextbus.VehicleLocation,
	out chan<- []*nextbus.VehicleLocation) {
	if !(0 < batchSize && batchSize <= pendingLimit) {
		panic(fmt.Errorf("batchSize=%v   pendingLimit=%v", batchSize, pendingLimit))
	}
	var pending []*nextbus.VehicleLocation
	for {
		//		log.Printf("StoreVehicleLocationsByDay ready to read")
		loc, ok := <-in
		// Has channel been closed?
		if !ok {
			break // Yup
		}
		pending = append(pending, loc)
		//		log.Printf("Have %v pending locations to sort", len(pending))
		if len(pending) < pendingLimit {
			continue
		}
		//		log.Printf("Sorting %v locations", len(pending))
		nextbus.SortVehicleLocationsByDateAndId(pending)
		out <- pending[0:batchSize]
		pending = pending[batchSize:]
	}
	if len(pending) > 0 {
		//		log.Printf("Sorting %v remaining locations", len(pending))
		nextbus.SortVehicleLocationsByDateAndId(pending)
		out <- pending
	}
	close(out)
}

type DayReportWriter struct {
	rootDirectory string
	currentDay    string
	currentPath   string
	file          *os.File
	buf           *bufio.Writer
	gzip          *gzip.Writer
	csv           *csv.Writer
	numLocations  int
}

func (p *DayReportWriter) Write(loc *nextbus.VehicleLocation) {
	day := loc.Time.Format("2006/01/02")
	if day != p.currentDay {
		p.Close()
		p.currentDay = day
		// Don't open the file if it already exists
		// (i.e. send location reports to the bit bucket).
		p.currentPath = filepath.FromSlash(filepath.Join(p.rootDirectory, day+".csv.gz"))
		if Exists(p.currentPath) {
			log.Printf("Already exists: %v", p.currentPath)
		} else if false {
			log.Printf("Would write to: %v", p.currentPath)
		} else {
			err := os.MkdirAll(filepath.Dir(p.currentPath), os.FileMode(0755))
			if err != nil {
				panic(err)
			}
			p.file, err = os.OpenFile(p.currentPath, os.O_CREATE, os.FileMode(0644))
			if err != nil {
				panic(err)
			}
			p.buf = bufio.NewWriterSize(p.file, 128*1024)
			p.gzip, err = gzip.NewWriterLevel(p.buf, gzip.BestCompression)
			if err != nil {
				panic(err)
			}
			p.csv = csv.NewWriter(p.gzip)
			log.Printf("Writing to: %v", p.currentPath)
		}
	}
	if p.csv != nil {
		err := p.csv.Write(loc.ToCSVFields())
		if err != nil {
			panic(err)
		}
		p.numLocations++
	}
}

func (p *DayReportWriter) Close() {
	if p.csv != nil {
		p.csv.Flush()
		p.csv = nil
	}
	if p.gzip != nil {
		err := p.gzip.Close()
		p.gzip = nil
		if err != nil {
			log.Printf("Failed to flush gzip for: %v", p.currentPath)
		}
	}
	if p.buf != nil {
		err := p.buf.Flush()
		p.buf = nil
		if err != nil {
			log.Printf("Failed to flush buffer for: %v", p.currentPath)
		}
	}
	if p.file != nil {
		err := p.file.Close()
		p.file = nil
		if err != nil {
			log.Printf("Failed to close: %v", p.currentPath)
		} else {
			log.Printf("Done writing %d locations to: %v", p.numLocations, p.currentPath)
		}
	}
	p.currentDay = ""
	p.currentPath = ""
	p.numLocations = 0
}

func StoreVehicleLocationsByDay(
	w *DayReportWriter,
	in <-chan []*nextbus.VehicleLocation,
	done chan<- bool) {
	for {
		locations, ok := <-in
		if !ok {
			break // Closed
		}
		for _, loc := range locations {
			w.Write(loc)
		}
	}
	w.Close()
	close(done)
}

func SelectArchives(fromDir, destinationRootDirectory string) (
	inPaths []string) {
	files, err := ioutil.ReadDir(fromDir)
	if err != nil {
		log.Fatal(err)
		return
	}
	for _, elem := range files {
		if elem.IsDir() {
			continue
		}
		name := elem.Name()
		if !strings.HasSuffix(name, ".tar.gz") {
			log.Printf("Not an archive: %v", name)
			continue
		}
		name = strings.Replace(name, ".tar.gz", ".csv.gz", 1)
		name = strings.Replace(name, "-", string(filepath.Separator), -1)
		outPath := filepath.Join(destinationRootDirectory, name)
		//		log.Printf("Checking for output: %v", outPath)
		if Exists(outPath) {
			log.Printf("Exists already: %v\n", outPath)
			continue
		}
		inPath := filepath.Join(fromDir, elem.Name())
		inPaths = append(inPaths, inPath)
	}
	sort.Strings(inPaths)
	// Discard the last archive so that we can pickup additional
	// observations for the day in the next day's first few reports.
	if len(inPaths) > 0 {
		log.Printf("Won't read latest archive: %v", inPaths[len(inPaths)-1])
		inPaths = inPaths[0 : len(inPaths)-1]
	}
	return
}

func main() {
	flag.Parse()

	max := runtime.GOMAXPROCS(-1)
	fmt.Println("Original GOMAXPROCS:", max)
	cpus := runtime.NumCPU()
	fmt.Println("NumCPU:", cpus)
	max = runtime.GOMAXPROCS(cpus)
	max = runtime.GOMAXPROCS(-1)
	fmt.Println("Current GOMAXPROCS:", max)

	archivePaths := SelectArchives(*from_dir, *to_dir)
	if len(archivePaths) == 0 {
		log.Print("No new archives to process")
		return
	}

	c1 := make(chan ParsedVehicleLocations, 10)
	c2 := make(chan *nextbus.VehicleLocation, 10)
	c3 := make(chan []*nextbus.VehicleLocation, 10)
	done := make(chan bool)

	w := &DayReportWriter{rootDirectory: *to_dir}

	go StoreVehicleLocationsByDay(w, c3, done)
	go SortAndBatchVehicleLocations(1000, 500, c2, c3)
	go ProcessParsedVehicleLocations(c1, c2)

	for _, archivePath := range archivePaths {
		//		log.Printf("Preparing to read: %v", archivePath)
		err := ReadArchive(archivePath, c1)
		if err != nil {
			log.Printf("ERROR: %v\n\tReading: %v", err, archivePath)
			break
		}
		//		log.Printf("Finished reading : %v", archivePath)
	}
	log.Print("Done reading from archives")
	close(c1)

	// Wait for the writing of the files to be done.
	_, ok := <-done
	log.Printf("Finished writing day's reports (%v)", ok)
}
