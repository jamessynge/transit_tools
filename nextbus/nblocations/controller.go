package nblocations

import (
"flag"
"github.com/golang/glog"
"os"
"path/filepath"
"time"
"util"
)

// DEBUG flags:

var debugArchivingFlag = flag.Bool(
	"debug_archiving", false,
	"DEBUG: instead of creating one tar per day, create a new one much more frequently.")

// TODO Maybe add support for web access to these objects; e.g. to the
// aggregator (latest location of all vehicles); stats on frequency; which
// files are being written to currently; etc.

// Start the process of fetching vehicle location reports, archiving
// the raw reports and aggregating them also into CSV files.
// Stops when it receives a |chan bool| on stopFetchAndArchiveCh; after
// stopping it sends true back on the channel it received.
func StartFetchAndArchive(
		agency string,
		interval time.Duration,
		extraSecs uint,
		fetcher util.HttpFetcher,
		agencyRootDir string,
		stopFetchAndArchiveCh chan chan bool) {
	// Root directory for saved vehicleLocations responses: compressed tar file
	// of xml responses (almost raw: we add a comment with metadata about the
	// request and response; and for failures, we store files of other types).
	rawRootDir := filepath.Join(agencyRootDir, "locations", "raw")
	if !util.IsDirectory(rawRootDir) {
		if err := os.MkdirAll(rawRootDir, 0755); err != nil {
			glog.Fatalf("Unable to MkdirAll(%q)!\nError: %s", rawRootDir, err)
		}
		glog.V(1).Infof("Created directory %s", rawRootDir)
	}
	// Root directory for the aggregated vehicle locations: daily (by default)
	// compressed CSV files.
	processedRootDir := filepath.Join(agencyRootDir, "locations", "processed")
	if !util.IsDirectory(processedRootDir) {
		if err := os.MkdirAll(processedRootDir, 0755); err != nil {
			glog.Fatalf("Unable to MkdirAll(%q)!\nError: %s", processedRootDir, err)
		}
		glog.V(1).Infof("Created directory %s", processedRootDir)
	}
	state := &locationFetchAndArchiveState{
		agency: agency,
		agencyRootDir: agencyRootDir,
		rawRootDir: rawRootDir,
		processedRootDir: processedRootDir,
		interval: interval,
		extraSecs: extraSecs,
		fetcher: fetcher,
		vlrArchiverInputCh: make(chan *VehicleLocationsResponse, 10),
		vlrArchiverStopCh: make(chan chan bool),
		csvArchiverInputCh: make(chan *VehicleLocationsResponse, 10),
		csvArchiverStopCh: make(chan chan bool),
		splitterInputCh: make(chan *VehicleLocationsResponse, 10),
		periodicFetcherStopCh: make(chan chan bool),
		primaryStopCh: stopFetchAndArchiveCh,
	}

	go state.RunVLRArchiver()
	go state.RunAggegator()
	go state.RunSplitter()

	go PeriodicFetcher(agency, interval, extraSecs, fetcher,
	                   state.periodicFetcherStopCh, state.splitterInputCh)

	go state.RunCleaner()

	return
}

func (p *locationFetchAndArchiveState) RunVLRArchiver() {
	dta := &util.DatedTarArchiver{
		RootDir: p.rawRootDir,
	}
	if *debugArchivingFlag {
		dta.PathFragmentLayout = filepath.Join("2006", "01", "02", "15", "2006-01-02_1504")
	} else {
		dta.PathFragmentLayout = filepath.Join("2006", "01", "2006-01-02")
	}
	archiver := NewVLRArchiver(dta)
	errorCount := 0
	for {
		select {
		case stoppedCh := <- p.vlrArchiverStopCh:
			glog.Info("Closing VLRArchiver...")
			if err := archiver.Close(); err != nil {
				glog.Errorln("Error during closing VLRArchiver:", err)
			}
			stoppedCh <- true
			return
		case vlr, ok := <- p.vlrArchiverInputCh:
			if !ok {
				p.vlrArchiverInputCh = nil
				continue
			}
			if err := archiver.AddResponse(vlr); err != nil {
				glog.Errorln("Error while archiving response:", err)
				errorCount++
				if errorCount > 10 {
					glog.Fatalf("Too many sequential errors (%d) archiving responses",
											errorCount)
				}
			} else {
				errorCount = 0
			}
		}
	}
}

type locationFetchAndArchiveState struct {
	agency string
	agencyRootDir string
	rawRootDir string
	processedRootDir string

	interval time.Duration
	extraSecs uint
	fetcher util.HttpFetcher

//	vlrArchiver *VLRArchiver
	vlrArchiverInputCh chan *VehicleLocationsResponse
	vlrArchiverStopCh chan chan bool
//	csvArchiver *VLRArchiver
	csvArchiverInputCh chan *VehicleLocationsResponse
	csvArchiverStopCh chan chan bool

	// Periodic fetcher's input is a timer, and its output goes to the splitter.
	// When we stop the periodic fetcher, it will close the splitterInputCh,
	// which will propagate to the other input channels.
	splitterInputCh chan *VehicleLocationsResponse
	periodicFetcherStopCh chan chan bool

	primaryStopCh chan chan bool
}

func (p *locationFetchAndArchiveState) RunAggegator() {
	var splitterOpener ArchiveSplitterOpener
	if *debugArchivingFlag {
		splitterOpener = MakeDebugArchiveSplitterOpener(p.processedRootDir)
	} else {
		splitterOpener = MakeDailyArchiveSplitterOpener(p.processedRootDir)
	}
	archiver := MakeCSVArchiver(splitterOpener, splitterOpener)
	aggregator := MakeVehicleAggregator()

	doClose := func() {
		if aggregator != nil {
			if err := archiver.WriteLocations(aggregator.Close()); err != nil {
				glog.Errorln("Error writing locations to CSV Archive", err)
			}
			aggregator = nil
		}
		if archiver != nil {
			if err := archiver.Close(); err != nil {
				glog.Errorln("Error closing CSV Archiver", err)
			}
			archiver = nil
		}
	}

	ticker := time.NewTicker(10 * time.Minute)
	if *debugArchivingFlag {
		ticker = time.NewTicker(25 * time.Second)
	}

	for {
		select {
		case stoppedCh := <- p.csvArchiverStopCh:
			glog.Info("CSV Archiver closing")
			if aggregator != nil {
				doClose()
			}
			stoppedCh <- true
			return
		case <- ticker.C:
			archiver.PartialFlush()
		case vlr, ok := <- p.csvArchiverInputCh:
			if !ok {
				doClose()
				continue
			}
			if vlr.Report == nil || len(vlr.Report.VehicleLocations) == 0 {
				continue
			}
			glog.V(1).Infof("Found %d vehicle updates",
											len(vlr.Report.VehicleLocations))
			aggregator.Insert(vlr.Report.VehicleLocations)
			locations := aggregator.RemoveStaleReports()
			if err := archiver.WriteLocations(locations); err != nil {
				glog.Errorln("Error writing locations to CSV Archive", err)
			}
		}
	}
}

func (p *locationFetchAndArchiveState) RunSplitter() {
	// Allow for an arbitrary number of output channels.
	writeTo := []chan *VehicleLocationsResponse{
		p.csvArchiverInputCh,
		p.vlrArchiverInputCh,
	}
	for {
		if vlr, ok := <- p.splitterInputCh; !ok {
			// Unable to read. Close the channels so the receivers
			// find out there is no more data coming.
			for _, ch := range writeTo {
				close(ch)
			}
			return
		} else {
			for _, ch := range writeTo {
				ch <- vlr
			}
		}
	}
}

func (p *locationFetchAndArchiveState) RunCleaner() {

	stopOne := func(name string, ch chan chan bool, timeout time.Duration) {
		glog.Infoln("Stopping", name)
		stoppedCh := make(chan bool)
		ch <- stoppedCh
		t := time.NewTimer(timeout)
		select {
		case <- stoppedCh:
			t.Stop()
			glog.Infoln("Stopped", name)
		case <- t.C:
			glog.Warningln(name, "did not respond within", timeout)
		}
	}

	waitForEmpty := func(name string, inputCh chan *VehicleLocationsResponse,
	                     timeout time.Duration) {
		start := len(inputCh)
		if start > 0 {
			glog.Infoln("Waiting until", name, "has received the", start, "pending input(s).")
			t := time.NewTimer(timeout)
			ticker := time.NewTicker(20 * time.Millisecond)
			for {
				num := 0
				select {
				case <- t.C:
					num = len(inputCh)
					if num == 1 {
						glog.Warningln("1 input remains for", name, "after waiting", timeout)
						return
					} else if num > 1 {
						glog.Warningln(num, "inputs remain for", name, "after waiting", timeout)
						return
					}
				case <- ticker.C:
					num = len(inputCh)
				}
				if num <= 0 {
					glog.Infoln(name, "has emptied its input channel.")
					return
				}
			}
		}
	}

	// Wait until requested to cleanup.
	stoppedCh := <- p.primaryStopCh

	// Tell the periodic fetcher to stop fetching; it will close its output
	// channel, which will propagate to the splitter's output channels once it
	// has drained its input channel.
	stopOne("Periodic Fetcher", p.periodicFetcherStopCh, 20 * time.Second)
	waitForEmpty("VLR Splitter", p.splitterInputCh, 20 * time.Second)

	waitForEmpty("VLR Archiver", p.vlrArchiverInputCh, 20 * time.Second)
	stopOne("VLR Archiver", p.vlrArchiverStopCh, 20 * time.Second)

	waitForEmpty("CSV Archiver", p.csvArchiverInputCh, 20 * time.Second)
	stopOne("CSV Archiver", p.csvArchiverStopCh, 20 * time.Second)

	// All done.
	stoppedCh <- true
}
