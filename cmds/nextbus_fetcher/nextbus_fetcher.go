// Program for fetching a transit agency's vehicle locations and route
// configurations and schedules using Nextbus's webservice api.  Fetches
// vehicle locations every N seconds (>= 10 seconds, the minimum interval
// per the Nextbus spec), saving the result to a directory of raw xml files,
// and also appending the contained records to a csv file.
// Periodically (e.g. daily) fetches the agency's "static" route and schedule
// information, which in practice may change anywhere from every few months to
// much more often.
// Based on fetch_vehicles5.go, which did not fetch the route and schedule
// information.
//
// TODO Compare static data vs previous day's data, so we can detect when
// there is a change in static data.
//
// TODO Consider whether it is better to have more simpler programs (e.g.
// one that just fetches locations and writes each response to an xml file;
// another that produces a tar of the xml files; another that produces the
// aggregated locations, i.e. csv.gz files).  It means more programs are
// running, but perhaps is more tolerant of failures.
//
// TODO Add rate limiting to avoid exceeding provider specified limits
// across all endpoints (e.g. qps, bps).  For NextBus, the limits are:
// * Maximum characters per requester for all commands (IP address): 2MB/20sec
// * Maximum routes per "routeConfig" command: 100
// * Maximum stops per route for the "predictionsForMultiStops" command: 150
// * Maximum number of predictions per stop for prediction commands: 5
// * Maximum timespan for "vehicleLocations" command: 5min
// * Maximum frequency for "vehicleLocations" command: 1 per 10sec

package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"github.com/jamessynge/transit_tools/nextbus/configfetch"
	"github.com/jamessynge/transit_tools/nextbus/nblocations"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"github.com/jamessynge/transit_tools/util"
)

var storageRootFlag = flag.String(
	"storage_root", "",
	"Root directory in which to store the agency directory, and under that " +
	"the raw, csv and log directories.")

// "mbta" is my primary interest, but "ccrta" is convenient because it is a
// small agency so the amount of data to be fetched is relatively small.
var agencyFlag = flag.String(
	"agency", "",
	"Transit agency/organization for which to fetch vehicle locations and " +
	"route schedules and configurations.")

var setLogDirFlag = flag.Bool(
	"set_log_dir", true,
	"Set --log_dir default value immediately after parsing flags so that " +
	"messages are logged there immediately, and not after the file is rotated.")

var fetchIntervalFlag = flag.Float64(
	"fetch_interval", 0,
	"Seconds between fetches of vehicle locations")
var extraDurationFlag = flag.Uint(
	"extra_duration", 60,
	"Seconds to subtract from last fetch time, in a probably vain attempt to " +
	"get more accuracy in the time of last report for a vehicle")

// Rate limit flag defaults set to 100,000 per second, just under 2MB/20s,
// the limit allowed by NextBus, but expressed such that the max available will
// be 100,000 so we don't flood NextBus when doing the daily config fetch
// after a period of just fetching locations, which aren't very big (for the
// MBTA, under 100KB is normal, and that is at most once every 10s).
var rateLimitBytesFlag = flag.Uint(
	"rate_limit_bytes", 100000,
	"Maximum number of bytes to fetch in --rate_limit_interval")
var rateLimitIntervalFlag = flag.Duration(
	"rate_limit_interval", 1 * time.Second,
	"Time period (e.g. 1s) over which to permit --rate_limit_bytes to be " +
	"exchanged with NextBus")

// Define a type, hoursSlice, that satisfies the flag.Value interface
type hoursSlice []int
func (p *hoursSlice) String() string {
    return fmt.Sprint(*p)
}
// Split the comma-separated list of integer hours (0 to 23) in the string.
func (p *hoursSlice) Set(value string) error {
	// Delete whatever value is already present.
	*p = make(hoursSlice, 0, 24)
	if value == "all" {
		for n := 0; n < 24; n++ {
			*p = append(*p, n)
		}
		return nil
	}
	for _, v := range strings.Split(value, ",") {
		u, err := strconv.ParseUint(v, 10, 5)
		if err != nil {
			return err
		} else if !(0 <= u && u < 24) {
			return fmt.Errorf("In valid hour number: %d", u)
		}
		*p = append(*p, int(u))
	}
	// TODO Eliminate duplicates.
	sort.Ints(*p)
	return nil
}
var configHours hoursSlice
func init() {
	// Tie the command-line flag to the configHours variable and
	// set a usage message.
	flag.Var(&configHours, "config_hours",
			"Comma-separated list of the hours of the day (0 to 23) at which " +
			"to fetch route schedules and configurations.")
}

/*
var http_port_flag = flag.Uint(
	"http_port", 8080,
	"Port for serving status as HTTP pages.")
var gob_port_flag = flag.Uint(
	"gob_port", 0,
	"Port for serving the current location of buses as Go's GOBs")
*/

type priorityFetcher struct {
	hlhf util.HiLoHttpFetcher
	hiPriority bool
}

func (p *priorityFetcher) Do(request *http.Request) (
		*util.HttpFetchResponse, error) {
	return p.hlhf.Do(p.hiPriority, request)
}

func (p *priorityFetcher) Close() {
	p.hlhf.Close()
	p.hlhf = nil
}

func noRedirect(req *http.Request, via []*http.Request) error {
	return errors.New("redirect not supported")
}

func CreateHiLoHttpFetcher(regulator util.RateRegulator) util.HiLoHttpFetcher {
	// The high priority requests should timeout fairly quickly because we want
	// to service the next request soon.
	hiClient := util.NewClientAndTransport()
	hiClient.CheckRedirect = noRedirect
	hiClient.Timeout = 9500 * time.Millisecond
	hiFetcher := util.NewHttpRegulatedFetcher(hiClient, regulator, false)

	// The low priority requests don't need to have a special timeout.
	loClient := util.NewClientAndTransport()
	loClient.CheckRedirect = noRedirect
	loClient.Transport.(*http.Transport).DisableKeepAlives = true
	loFetcher := util.NewHttpRegulatedFetcher(hiClient, regulator, false)

	return util.NewHiLoHttpFetcher2(hiFetcher, loFetcher)
}

func main() {
	//	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	flag.Parse()

	if len(*storageRootFlag) == 0 {
		glog.Fatal("Must specify -storage_root directory")
	}
	if len(*agencyFlag) == 0 {
		glog.Fatal("Must specify -agency=<NextBusAgencyName>")
	}

	agencyDir := filepath.Join(*storageRootFlag, *agencyFlag)
	if !util.IsDirectory(agencyDir) {
		if err := os.MkdirAll(agencyDir, 0755); err != nil {
			glog.Fatalf("Unable to MkdirAll(%q)!\nError: %s", agencyDir, err)
		}
	}

	// Set --log_dir before any logging with glog (except the Fatal calls above)
	// so that the log files are in the correct location.
	if *setLogDirFlag {
		util.SetDefaultLogDir(filepath.Join(agencyDir, "logs"))
	}
	// Flush the files when shutting down
	defer glog.Flush()

	// Root directory for the semi-static agency route information: list of
	// routes, per route schedule and path.
	// TODO Create a new config dir each day?
	configRootDir := filepath.Join(agencyDir, "config")
	if !util.IsDirectory(configRootDir) {
		if err := os.MkdirAll(configRootDir, 0755); err != nil {
			glog.Fatalf("Unable to MkdirAll(%q)!\nError: %s", configRootDir, err)
		}
		glog.V(1).Infof("Created directory %s", configRootDir)
	}

	util.InitGOMAXPROCS()

	// How often should we fetch vehicle locations?
	if 0 < *fetchIntervalFlag && *fetchIntervalFlag < 10 {
		glog.Fatalf("The specified fetch interval (%f) is too short", *fetchIntervalFlag)
	}
	if *fetchIntervalFlag < 10 {
		const minInterval = 10.0
		fetchInterval := float64(minInterval)
		if *extraDurationFlag > 0 {
			totalDuration := minInterval + float64(*extraDurationFlag)
			times := totalDuration / minInterval
			if times > 2 {
				fetchInterval = fetchInterval + 1.0/times
			}
		}
		if *fetchIntervalFlag != 0 {
			glog.Infof("Changing fetchInterval from %f to %f", *fetchIntervalFlag, fetchInterval)
		}
		*fetchIntervalFlag = fetchInterval
	}

	// Start rate regulator to limit the rate at which we send to/receive from
	// the NextBus api service.
	if (uint32(*rateLimitBytesFlag) <= 0) {
		glog.Fatal("Must specify a positive value for --rate_limit_bytes")
	}
	if (*rateLimitIntervalFlag <= 0) {
		glog.Fatal("Must specify a positive value for --rate_limit_interval")
	}
	regulator, err := util.NewRateRegulator(
			0, uint32(*rateLimitBytesFlag), *rateLimitIntervalFlag)
	if err != nil {
		glog.Fatal("Failed to create rate regulator: ", err)
	}

	// Start regulated HTTP fetcher with two priority levels, a high one for the
	// vehicle location requests, and a low one for the config requests.
	hlhf := CreateHiLoHttpFetcher(regulator)
	lohf := &priorityFetcher{
		hlhf: hlhf,
		hiPriority: false,
	}
	hihf := &priorityFetcher{
		hlhf: hlhf,
		hiPriority: true,
	}

	// Start fetching, archiving and aggregating of vehicle locations.
	interval := time.Duration(*fetchIntervalFlag * 1000000) * time.Microsecond
	stopFetchAndArchiveCh := make(chan chan bool, 1)
	nblocations.StartFetchAndArchive(*agencyFlag, interval, *extraDurationFlag,
																	 hihf, agencyDir, stopFetchAndArchiveCh)
	glog.Info("Started location fetcher.")

	// Start fetching the semi-static agency configuration data.
	stopPCFCh := make(chan chan bool, 1)
	go configfetch.PeriodicConfigFetcher(*agencyFlag, configRootDir, lohf,
																			 []int(configHours), stopPCFCh)
	glog.Info("Started config fetcher.")

	// TODO Start http server to report status (e.g. names of files currently
	// being written to).  Maybe also use http server to change flags?
	/*
	   // Default Request Handler
	   func defaultHandler(w http.ResponseWriter, r *http.Request) {
	   	fmt.Fprintf(w, "<h1>Hello %s!</h1>", r.URL.Path[1:])
	   }
	   	http.HandleFunc("/", defaultHandler)
	   	http.ListenAndServe(":8080", nil)
	*/

	// Set up channel on which to receive signal notifications (e.g. for
	// shutting down). We must use a buffered channel or risk missing the
	// signal if we're not ready to receive when the signal is sent.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT,
	              syscall.SIGTERM, syscall.SIGQUIT)

	sig := <-signalChan
	switch sig {
	case syscall.SIGHUP:
		glog.Info("syscall.SIGHUP")
	case syscall.SIGINT:
		glog.Info("syscall.SIGINT")
	case syscall.SIGTERM:
		glog.Info("syscall.SIGTERM")
	case syscall.SIGQUIT:
		glog.Info("syscall.SIGQUIT")
	}

	// Stop the fetchers.
	glog.Info("Stopping location fetcher.")
	stoppedCh1 := make(chan bool)
	stopFetchAndArchiveCh <- stoppedCh1

	glog.Info("Stopping config fetcher.")
	stoppedCh2 := make(chan bool)
	stopPCFCh <- stoppedCh2

	glog.Flush()

	// Wait for them to finish.
	for stoppedCh1 != nil || stoppedCh2 != nil {
		select {
		case <- stoppedCh1:
			glog.Info("Stopped location fetcher.")
			stoppedCh1 = nil
		case <- stoppedCh2:
			glog.Info("Stopped config fetcher.")
			stoppedCh2 = nil
		case sig = <- signalChan:
			switch sig {
			case syscall.SIGHUP:
				glog.Info("syscall.SIGHUP")
			case syscall.SIGINT:
				glog.Info("syscall.SIGINT")
			case syscall.SIGTERM:
				glog.Info("syscall.SIGTERM")
			case syscall.SIGQUIT:
				glog.Info("syscall.SIGQUIT")
			}
			os.Exit(1)
		}
	}

	os.Exit(0)
}
