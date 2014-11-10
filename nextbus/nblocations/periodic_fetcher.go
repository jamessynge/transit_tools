package nblocations

import (
	"github.com/golang/glog"
	"net/http"
	"nextbus"
	"time"
	"util"
)

type VehicleLocationsResponse struct {
	Agency   string
	Url      string
	LastTime time.Time
	// lastTime value from previous request, which was
	// used to generate query parameter t in Url.
	LastLastTime time.Time
	// Our time just before sending request to server.
	RequestTime time.Time
	// Our time just after receiving the headers (i.e. before reading body, but
	// after we got the Date header).
	ResultTime time.Time
	Response   *http.Response
	ServerTime time.Time
	Body       []byte
	Report     *nextbus.VehicleLocationsReport
	Error      error
}

func fetchOnce(
	agency string, lastTime time.Time, extraSecs uint,
	httpFetcher util.HttpFetcher) (
	*VehicleLocationsResponse, error) {
	url, t := UrlAndT(agency, lastTime, extraSecs)
	hr, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	hfr, err := httpFetcher.Do(hr)
	if hfr == nil && err != nil {
		return nil, err
	}
	vlr := &VehicleLocationsResponse{
		Agency:       agency,
		Url:          url,
		LastTime:     util.UnixMillisToTime(t),
		LastLastTime: lastTime,
		RequestTime:  hfr.StartTime,
		ResultTime:   hfr.ResponseTime,
		Response:     hfr.Response,
		Body:         hfr.Body,
		Error:        err,
	}
	diff := vlr.ResultTime.Sub(vlr.RequestTime)
	vlr.ServerTime = vlr.RequestTime.Add(diff / time.Duration(2))
	if vlr.Response != nil {
		if serverTime, found := util.GetServerTime(vlr.Response); found {
			vlr.ServerTime = serverTime
		}
		if vlr.Response.StatusCode == http.StatusOK {
			if !BodyIsXml(vlr) {
				glog.Warningf("Unexpected content type: %s", util.GetContentType(
					vlr.Response, vlr.Body))
			} else {
				vlr.Report, err = nextbus.ParseXmlVehicleLocations(vlr.Body)
				if err != nil {
					glog.Warningf("Error from ParseXmlVehicleLocations: %s", err)
					if vlr.Error == nil {
						vlr.Error = err
					}
				} else {
					glog.V(1).Infof("Found %d vehicle updates", len(vlr.Report.VehicleLocations))
				}
			}
		}
	}
	return vlr, vlr.Error
}

func PeriodicFetcher(agency string, interval time.Duration, extraSecs uint,
										 httpFetcher util.HttpFetcher, stopCh <-chan chan bool,
										 responseCh chan<- *VehicleLocationsResponse) {
	glog.Infof("agency=%q, interval=%s", agency, interval)
	lastTime := util.UnixMillisToTime(0)
	intervalTicker := time.NewTicker(interval)
	var shortDuration time.Duration = 0
	shortTimer := time.NewTimer(time.Duration(1) * time.Millisecond)

	exec := func() {
		if shortDuration != 0 {
			// Recovering.  In hopes that we recover this time, start a new Ticker
			// from the start of this fetch.
			intervalTicker = time.NewTicker(interval)
			glog.Infof("shortTimer expired, duration: %s", shortDuration)
		}
		good := true
		vlr, err := fetchOnce(agency, lastTime, extraSecs, httpFetcher)
		if vlr == nil {
			good = false
			glog.Errorf("Complete failure fetching '%s' vehicle location\nError: %s",
				agency, err)
		} else if vlr.Report != nil {
			if vlr.Error != nil {
				good = false
				glog.Errorf(
					"Partial failure fetching vehicle locations from %s\nError: %s",
					vlr.Url, vlr.Error)
			}
			if vlr.Report.LastTime.After(lastTime) {
				lastTime = vlr.Report.LastTime
				glog.V(1).Infof("Updated lastTime to %s", lastTime)
			} else if vlr.Report.LastTime.Unix() == 0 {
				good = false
			} else if vlr.Report.LastTime.Before(lastTime) {
				glog.Warningf("lastTime going backwards, latest report is %s behind",
					lastTime.Sub(vlr.Report.LastTime))
				// This can happen when the server adjusts its time.  I don't know
				// a good way to deal with this (yet).  Perhaps I should keep track
				// of what I think the server time should be (e.g. local clock delta
				// should be added to previous server time, and if that is very
				// different from the current server time then we consider that a
				// time adjustment has occurred).
				//good = false
			}
		} else {
			good = false
			if vlr.Error != nil {
				glog.Errorf("Failed to fetch vehicle locations from %s\nError: %s",
					vlr.Url, vlr.Error)
			} else {
				glog.Errorf("Failed to fetch vehicle locations from %s",
					vlr.Url)
			}
		}
		responseCh <- vlr
		if good {
			if shortDuration != 0 {
				glog.Infof("Recovered from fetch errors, resuming normal ticking")
				intervalTicker = time.NewTicker(interval)
			}
			shortDuration = 0
			return
		}
		// Need to recover.  Stop the ticker, and compute how long until we can
		// try again.
		intervalTicker.Stop()
		if shortDuration == 0 {
			// Next exec will be our first recovery attempt following a failure.
			shortDuration = time.Duration(1) * time.Second
		} else {
			shortDuration *= 2
			if shortDuration > interval {
				shortDuration = interval
			}
		}
		shortTimer.Reset(shortDuration)
		return
	}

	for {
		select {
		case stoppedCh := <-stopCh:
			if intervalTicker != nil {
				intervalTicker.Stop()
			}
			if shortTimer != nil {
				shortTimer.Stop()
			}
			close(responseCh)
			stoppedCh <- true
			return
		case <-shortTimer.C:
			exec()
		case <-intervalTicker.C:
			glog.V(1).Infof("Tick")
			// TODO Maybe split this up so that I add another case to this select
			// statement for receiving the response, and maybe even another one
			// for a timeout of the fetch.
			exec()
		}
	}
}

func BodyIsXml(vlr *VehicleLocationsResponse) bool {
	return vlr != nil && util.BodyIsXml(vlr.Response, vlr.Body)
}

func BodyIsHtml(vlr *VehicleLocationsResponse) bool {
	return vlr != nil && util.BodyIsHtml(vlr.Response, vlr.Body)
}
