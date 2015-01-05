package nblocations

import (
	"net/http"
	"time"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
)

// Average of RequestTime and ResultTime
func (vlr *VehicleLocationsResponse) EstimatedServerTime() time.Time {
	diff := vlr.ResultTime.Sub(vlr.RequestTime)
	return vlr.RequestTime.Add(diff / time.Duration(2))
}

// Difference between the time reported by the server and our time.
func (vlr *VehicleLocationsResponse) ServerTimeOffset() time.Duration {
	return vlr.ServerTime.Sub(vlr.EstimatedServerTime())
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
	if vlr.Response != nil {
		if serverTime, found := util.GetServerTime(vlr.Response); found {
			vlr.ServerTime = serverTime
		} else {
			// Making this a warning just so that I can find it more easily.
			// Perhaps this would be better as or with the addition of a varz counter.
			glog.Warning("Server didn't return Date header")
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

	// Setup a timer used for recovery, but stop it before it fires (we need it
	// setup for the channel on which we'll wait).
	var shortDuration time.Duration = 0
	shortTimer := time.NewTimer(time.Duration(1) * time.Hour)
	shortTimer.Stop()

	// And start the ticker which will trigger the normal fetching (except during
	// recovery following a fetch failure).
	intervalTicker := time.NewTicker(interval)

	exec := func() (retryFetch bool, vlr *VehicleLocationsResponse) {
		var err error
		vlr, err = fetchOnce(agency, lastTime, extraSecs, httpFetcher)
		if vlr == nil {
			glog.Errorf("Complete failure fetching '%s' vehicle location\nError: %s",
				agency, err)
			return true, nil
		}
		if vlr.Report == nil {
			if vlr.Error == nil {
				glog.Errorf("Failed to fetch vehicle locations from %s",
					vlr.Url)
			}
			return true, vlr
		}
		if vlr.Error != nil {
			glog.Errorf(
				"Partial failure fetching vehicle locations from %s\nError: %s",
				vlr.Url, vlr.Error)
		}
		if vlr.Report.LastTime.IsZero() {
			retryFetch = true
			glog.V(1).Infof("No lastTime in response")
		} else if vlr.Report.LastTime.After(lastTime) {
			lastTime = vlr.Report.LastTime
			glog.V(1).Infof("Updated lastTime to %s", lastTime)
		} else if vlr.Report.LastTime.Before(lastTime) {
			glog.Warningf("lastTime going backwards, latest report is %s behind",
				lastTime.Sub(vlr.Report.LastTime))
			// This can happen when the server adjusts its time.  I don't know
			// a good way to deal with this (yet).  Perhaps I should keep track
			// of what I think the server time should be (e.g. local clock delta
			// should be added to previous server time, and if that is very
			// different from the current server time then we consider that a
			// time adjustment has occurred).
		}
		return
	}

	doRetry := func() {
		// Recovering.  In hopes that we recover this time, start a new Ticker
		// from the start of this fetch.
		intervalTicker = time.NewTicker(interval)
		glog.Infof("shortTimer expired, duration: %s", shortDuration)
		retryFetch, vlr := exec()
		responseCh <- vlr
		if !retryFetch {
			glog.Infof("Recovered from fetch errors, resuming normal ticking")
			intervalTicker = time.NewTicker(interval)
			shortDuration = 0
			return
		}
		// Still trying to recover. Compute how long until we can try again.
		intervalTicker.Stop()
		shortDuration *= 2
		if shortDuration > interval {
			shortDuration = interval
		}
		shortTimer.Reset(shortDuration)
		return
	}

	// Handle the normal tick case (i.e. last fetch succeeded).
	doTick := func() {
		glog.V(1).Infof("Tick")
		retryFetch, vlr := exec()
		responseCh <- vlr
		if !retryFetch {
			// All went fine, done until next call.
			return
		}
		// Need to recover.  Stop the ticker, and try again in a second.
		intervalTicker.Stop()
		shortDuration = time.Duration(1) * time.Second
		shortTimer.Reset(shortDuration)
	}

	doTick()

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
			doRetry()
		case <-intervalTicker.C:
			doTick()
		}
	}
}

func BodyIsXml(vlr *VehicleLocationsResponse) bool {
	return vlr != nil && util.BodyIsXml(vlr.Response, vlr.Body)
}

func BodyIsHtml(vlr *VehicleLocationsResponse) bool {
	return vlr != nil && util.BodyIsHtml(vlr.Response, vlr.Body)
}
