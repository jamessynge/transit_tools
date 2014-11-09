package nblocations

import (
	"fmt"

	"github.com/golang/glog"
	"nextbus"
	"time"
	"util"
)

const (
	BASE_URL = nextbus.BASE_URL
)

func ComputeT(agency string, lastTime time.Time, extraSeconds uint) int64 {
	// Note that time.Since(t time.Time) is based on our clock, not the server's
	// clock, so the estimate of how old lastTime is may be considerably off.
	t := util.TimeToUnixMillis(lastTime)
	if t > 0 {
		if extraSeconds == 0 {
			// Not doing the fancy overlapping fetches.
			since := time.Since(lastTime)
			if since.Minutes() > 5 {
				// Nextbus says don't request more than 5 minutes back, but you can specify
				// t=0 and will get back as much as 15 minutes of data.
				t = 0
			}
		} else {
			extraDuration := time.Duration(-extraSeconds) * time.Second
			t2 := lastTime.Add(extraDuration)
			since := time.Since(t2)
			if since.Minutes() > 5 {
				// Limit fetches to the last 5 minutes, so we don't suddenly get old
				// location reports for vehicles we previously flushed from the
				// aggregator.
				t2 = time.Now().Add(time.Duration(-5) * time.Minute)
				glog.V(2).Infof("lastTime-extraSeconds is too old; lastTime adjusted\n  From: %s\n    To: %s", lastTime, t2)
			} else {
				glog.V(2).Infof("Adjusted lastTime by %s\n  From: %s\n    To: %s", extraDuration, lastTime, t2)
			}
			t = util.TimeToUnixMillis(t2)
		}
	} else if t < 0 {
		glog.Warningf("Why is time before unix epoch?")
		glog.Warningf("lastTime: %s", lastTime)
		glog.Warningf("t: %d", t)
		glog.Warningf("lastTime.Unix(): %d", lastTime.Unix())
		glog.Warningf("lastTime.Nanosecond(): %d", lastTime.Nanosecond())
		//		glog.Warningf("Why is time before unix epoch?\nlastTime: %s\nt: %d", lastTime, t)
		t = 0
	}
	return t
}

func Url(agency string, lastTime time.Time, extraSeconds uint) string {
	t := ComputeT(agency, lastTime, extraSeconds)
	return fmt.Sprintf("%s?command=vehicleLocations&a=%s&t=%d",
		BASE_URL, agency, t)
}

func UrlAndT(agency string, lastTime time.Time, extraSeconds uint) (
	string, int64) {
	t := ComputeT(agency, lastTime, extraSeconds)
	url := fmt.Sprintf("%s?command=vehicleLocations&a=%s&t=%d", BASE_URL, agency, t)
	return url, t
}
