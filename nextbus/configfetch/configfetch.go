package configfetch

import (
"bytes"
"fmt"
"net/http"
"path/filepath"
"sort"
"strings"
"time"
"os"

"github.com/golang/glog"

"github.com/jamessynge/transit_tools/nextbus"
"github.com/jamessynge/transit_tools/util"
)

func isErrorToSuppress(err error) bool {
	return strings.Contains(err.Error(),
													"Comparison method violates its general contract!")
}

type state struct {
	agency, rootDir string
	fetcher util.HttpFetcher
	errs util.Errors
	// When done, send collected errors (as a single error) to doneCh
	doneCh chan error
	// When received, set stop to true.
	stopCh chan chan bool
	stop bool
	// Stop response channel; send true when stopped.
	stopResponseCh chan bool
}

func (p *state) doWait(waitFor time.Duration) {
	if !p.stop {
		timer := time.NewTimer(waitFor)
		select {
		case rCh := <- p.stopCh:
			p.stop = true
			p.stopResponseCh = rCh
			timer.Stop()
			glog.Infof("Stopping config fetch for agency %q.", p.agency)
			return
		case <- timer.C:
			return
		}
	}
}

func (p *state) fetchOnce(url string) (*util.HttpFetchResponse, error) {
	glog.V(1).Infof("fetchOnce url: %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil { return nil, err }
	return p.fetcher.Do(req)
}

func (p *state) fetchWithRetries(command, routeTag string) (
		*util.HttpFetchResponse, *nextbus.BodyElement, error) {
	url := fmt.Sprintf("%s?command=%s&a=%s", nextbus.BASE_URL, command, p.agency)
	if len(routeTag) > 0 { url = fmt.Sprintf("%s&r=%s", url, routeTag) }
	p.doWait(0)	// Check whether we need to stop.
	var totalWait time.Duration
	for n := 1; n < 60 && !p.stop; n += n {
		resp, err := p.fetchOnce(url)
		if err == nil && resp != nil {
			// Parse the body and check for errors.
			var bodyElem *nextbus.BodyElement
			if bodyElem, err = nextbus.UnmarshalNextbusXml(resp.Body); err == nil {
				if len(bodyElem.Routes) == 0 && bodyElem.Error != nil {
					err = fmt.Errorf("Error from Nextbus (shouldRetry=%t): %s\n",
													 bodyElem.Error.ShouldRetry,
													 bodyElem.Error.ElementText)
					if isErrorToSuppress(err) {
						return resp, bodyElem, err
					}
				} else {
					return resp, bodyElem, err
				}
			}
		}
		glog.Warningf("fetchOnce failed, will wait %d seconds\nError: %s", n, err)
		waitFor := time.Duration(n) * time.Second
		p.doWait(waitFor)
		totalWait += waitFor
	}
	if p.stop {
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("Failed after waiting a total of %s", totalWait)
}

// Creates the directory that contains filepath if it doesn't exist.
func (p *state) writeFile(command, routeTag, ext string, fileBody [][]byte) (
		err error) {
	var path string
	if routeTag == "" {
		path = filepath.Join(p.rootDir, command + ext)
	} else {
		path = filepath.Join(p.rootDir, command, routeTag + ext)
	}
	err = os.MkdirAll(filepath.Dir(path), 0750)
	if err != nil {
		return err
	}
	err = util.WriteFile(path, fileBody, 0544)
	return
}

var standardNextbusHeaders = map[string]string{
	"Access-Control-Allow-Origin": "*",
	"Connection":                  "",
	"Content-Type":                "text/xml",
	"Keep-Alive":                  "",
	"Server":   		               "",
	"Vary":                        "Accept-Encoding",
	"X-Frame-Options":             "SAMEORIGIN",
}

func isStandardNextbusHeader(key, value string) bool {
	if v, ok := standardNextbusHeaders[key]; ok {
		return v == "" || v == value 
	}
	return false
}

// Saves the response body into a file named based on the command and routeTag.
// Inserts info from resp into a comment before the root element of the xml
// document. Only call if response body is xml.
func (p *state) saveXml(command, routeTag string, resp *util.HttpFetchResponse) error {
	whitespaceStart, rootOffset, err := util.FindRootXmlElementOffset(resp.Body)
	if err != nil {
		glog.Warningf("Failed to find XML root: %s", err)
		return err
	}
	var fileBody [][]byte
	if whitespaceStart > 0 {
		fileBody = append(fileBody, resp.Body[0:whitespaceStart])
	}
	var buf bytes.Buffer
	buf.WriteString("\n<!--\n")
	fmt.Fprintf(&buf, "URL=%s\n", resp.Response.Request.URL)
	resp.WriteSummary(&buf, isStandardNextbusHeader)
	buf.WriteString("-->\n")
	fileBody = append(fileBody, buf.Bytes())
	fileBody = append(fileBody, resp.Body[rootOffset:])
	return p.writeFile(command, routeTag, ".xml", fileBody)
}

func (p *state) fetchAndSave(command, routeTag string) (
		*util.HttpFetchResponse, *nextbus.BodyElement, bool) {
	hfr, bodyElem, err := p.fetchWithRetries(command, routeTag)
	if err != nil {
		if !isErrorToSuppress(err) {
			p.errs.AddError(err)
		}
		return nil, nil, false
	}
	if p.stop {
		return nil, nil, false
	}
	if hfr.Response.StatusCode != http.StatusOK {
		p.errs.AddError(
				fmt.Errorf("Expected status %d, not %d",
									 hfr.Response.StatusCode,
									 http.StatusOK))
		return nil, nil, false
	}
	p.errs.AddError(p.saveXml(command, routeTag, hfr))
	return hfr, bodyElem, true
}

func (p *state) fetchRouteList() (routeTags []string) {
	if _, bodyElem, ok := p.fetchAndSave("routeList", ""); ok {
		// Extract the route tags.
		for _, routeElem := range bodyElem.Routes {
			routeTags = append(routeTags, routeElem.Tag)
		}
		glog.V(1).Infoln("Fetched list of", len(routeTags), "routes")
	} else if !p.stop {
		glog.Errorln("Failed to fetch route list for agency '%s'", p.agency)
	}
	return
}

func (p *state) fetchRoute(routeTag string) {
	_, _, ok1 := p.fetchAndSave("routeConfig", routeTag)
	if p.stop {
		return
	} else if !ok1 {
		glog.Warningln("Failed to fetch routeConfig for routeTag", routeTag)
	}
	_, _, ok2 := p.fetchAndSave("schedule", routeTag)
	if p.stop {
		return
	} else if !ok2 {
		glog.Warningln("Failed to fetch schedule for routeTag", routeTag)
	} else if ok1 {
		glog.V(1).Infoln("Fetched routeTag", routeTag)
	}
}

func (p *state) fetchAll() {
	startTime := time.Now()

	// Fetch the list of routes first.
	routeTags := p.fetchRouteList()

	// For each routeTag:
	for _, routeTag := range routeTags {
		if p.stop { break }
		p.fetchRoute(routeTag)
	}

	// If asked to stop, let the requestor know that we've stopped.
	if p.stop {
		if p.stopResponseCh != nil {
			p.stopResponseCh <- true
		}
		p.stopResponseCh = nil
	} else {
		// Else let the originator know that we're done.
		glog.Infoln("Finished config fetch for", len(routeTags),
								"routes of", p.agency, "in", time.Since(startTime))
		if p.doneCh != nil {
			err := p.errs.ToError()
			p.doneCh <- err
		}
	}
}

func StartAgencyConfigFetcher(
		agency, rootDir string, fetcher util.HttpFetcher) (
		doneCh chan error, stopCh chan chan bool) {
	p := &state{
		agency: agency,
		rootDir: rootDir,
		fetcher: fetcher,
		errs: util.NewErrors(),
		stopCh: make(chan chan bool, 1),
		doneCh: make(chan error, 1),
	}

	go p.fetchAll()

	return p.doneCh, p.stopCh
}

// Fetches the configuration data for an agency, stores it in rootDir.
// Assumes rootDir contains any identifying information needed to distinguish
// multiple calls to this function, such as date, time or agency.
func FetchAgencyConfig(agency, rootDir string,
											 fetcher util.HttpFetcher) error {
	p := &state{
		agency: agency,
		rootDir: rootDir,
		fetcher: fetcher,
		errs: util.NewErrors(),
	}

	p.fetchAll()
	return p.errs.ToError()
}

func PeriodicConfigFetcher(agency, rootDir string, fetcher util.HttpFetcher,
													 fetchHours []int, stopPCFCh chan chan bool) {
	layout := filepath.Join("2006", "01", "02", "2006-01-02_1504")
	if len(fetchHours) == 0 {
		fetchHours = []int{5}
	} else {
		sort.Ints(fetchHours)
		checkHour := func(h int) {
			if h < 0 || 23 < h {
				glog.Fatal("Hours must be in the range [0, 23], not ", h)
			}
		}
		checkHour(fetchHours[0])
		checkHour(fetchHours[len(fetchHours) - 1])
	}
	glog.Infoln("Config data fetch hours:", fetchHours)

	// Channel on which to send in order to indicate that we've stopped;
	// set once we've been asked to stop.
	var stoppedPCFCh chan bool
	lastDir := ""

	fetchOnce := func() {
		currentDir := filepath.Join(rootDir, time.Now().Format(layout))
		configFetchDoneCh, stopConfigFetchCh := StartAgencyConfigFetcher(
			agency, currentDir, fetcher)
		// Channel to send to current config fetcher when we've been
		// asked to stop, so it can tell us when it has stopped...
		var configFetchStoppedCh chan bool
		for {
			select {
			case err := <- configFetchDoneCh:
				// Normal case: config fetch finished and reported its status.
				if err != nil {
					glog.Warning("Error fetching config for '", agency, "': ", err)
				} else {
					glog.V(1).Info("Fetched config for '", agency, "'")
				}
				// Compare currentDir against last dir, so that we can eliminate
				// intermediate config directories that are the same as those on
				// either side of them.
				// A HACK FOR NOW (i.e. no ability to stop it mid-compare).
				if lastDir != "" {
					eq, err := CompareConfigDirs(lastDir, currentDir)
					if eq && err == nil {
						// Can get rid of last dir as it is identical for our purposes.
						glog.Infof("Lasest configuration is the same as the last")
						glog.Infof("Removing the last config dir: %s", lastDir)
						err := os.RemoveAll(lastDir)
						if err != nil {
							glog.Errorf("Error will removing config dir: %s\nError: %s", lastDir, err)
						}
						// TODO If ancestor dir is now empty, delete it too.
					} else if err != nil {
						glog.Warningf(`Failed to compare config directories:
 Last: %s
 This: %s
Error: %s`, lastDir, currentDir, err)
					} else {
						// Supposedly a rare occurrence; we'll see.
						glog.Info("CONFIGURATIONS HAVE CHANGED!!!")
					}
				}
				lastDir = currentDir
				return
			case stoppedPCFCh = <- stopPCFCh:
				glog.Info("Stopping in-progress fetch of config for '", agency, "'")
				configFetchStoppedCh = make(chan bool)
				stopConfigFetchCh <- configFetchStoppedCh
				stopPCFCh = nil
			case <- configFetchStoppedCh:
				glog.Info("Stopped fetching config for '", agency, "'")
				return
			}
		}
	}

	fetchTimesOfDay := func(day time.Time) (result []time.Time) {
		for _, targetHour := range fetchHours {
			result = append(result, util.SnapToHourOfDay(day, targetHour))
		}
		return
	}

	fetchToday := func(fetchTimes []time.Time) {
		now := time.Now()
		for _, targetTime := range fetchTimes {
			// Allow for the possibility that we start in the middle of the day, or
			// that it takes more than an hour to fetch; for the MBTA, it appears
			// to take around 15 minutes to fetch the data when rate limited, with
			// concurrent vehicle location fetching.
			waitFor := targetTime.Sub(now)
			glog.V(1).Infoln("now:", now, "   targetTime:", targetTime)
			if waitFor <= 0 { continue }
			// Wait until target time is reached.
			// Not sure how this would handle daylight saving changes, where we
			// either have two instances of 1am we fall back, or no instance of
			// 2am when we spring forward.
			glog.Infoln("Waiting", waitFor, "until",
					util.PrettyFutureTime(now, targetTime), "for next config fetch.")
			t := time.NewTimer(waitFor)
			select {
			case <- t.C:
				// Done waiting, do another fetch.
				fetchOnce()
				if stoppedPCFCh != nil {
					return
				}
			case stoppedPCFCh = <- stopPCFCh:
				glog.Info("PeriodicConfigFetcher for '", agency, "' stopping")
				stopPCFCh = nil
				return
			}
			now = time.Now()
		}
	}

	// Do an immediate fetch upon startup.
	day := time.Now()
	fetchOnce()
	if stoppedPCFCh != nil {
		stoppedPCFCh <- true
		return
	}

	// Fetch every day at the designated times.
	for {
		fetchTimes := fetchTimesOfDay(day)
		fetchToday(fetchTimes)
		if stoppedPCFCh != nil {
			stoppedPCFCh <- true
			return
		}
		day = util.MidnightOfNextDay(fetchTimes[len(fetchTimes) - 1])
	}
}
