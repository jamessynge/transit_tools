package util
/*
On MS Windows, to track how well system time is tracking an NTP server, try
one of the following:
w32tm /stripchart /period:900 /computer:2.pool.ntp.org
w32tm /stripchart /period:900 /computer:time.nist.gov
*/
import (
	"flag"
	"fmt"
//	"strings"
	"time"
)

type TimeLocation struct {
	Location *time.Location
}
func (p *TimeLocation) At(t time.Time) time.Time {
	return t.In(p.Location)
}
func (p *TimeLocation) Set(s string) error {
	v, err := time.LoadLocation(s)
	if err != nil { return err }
	p.Location = v
	return nil
}
func (p *TimeLocation) String() string {
	if p.Location == nil { return "<nil-location>" }
	return p.Location.String()
}
func (p *TimeLocation) Get() interface{} {
	return p.Location
}
func NewTimeLocationFlag(name, defaultLocation, usage string) *TimeLocation {
	return NewTimeLocationFlagSet(flag.CommandLine, name, defaultLocation, usage)
}
func NewTimeLocationFlagSet(
		fs *flag.FlagSet, name, defaultLocation, usage string) *TimeLocation {
	p := new(TimeLocation)
	if defaultLocation != "" {
		val, err := time.LoadLocation(defaultLocation)
		if err != nil {
			// Since this method is (typically) called during an init() function,
			// before main has been called, it may not be OK to call glog yet,
			// so we'll just panic if the default is bad/unusable (e.g. if the
			// time.Location database is missing).
			panic(fmt.Sprintf("Unable to convert %s to a time.Location value!\n%s",
												defaultLocation, err))
		}
		p.Location = val
	}
	fs.Var(p, name, usage)
	return p
}

type TimeLocationPtr struct {
	pp **time.Location
}
func (p *TimeLocationPtr) Set(s string) error {
	v, err := time.LoadLocation(s)
	if err != nil { return err }
	*(p.pp) = v
	return nil
}
func (p *TimeLocationPtr) String() string {
	v := *(p.pp)
	if v == nil {
		return "<nil>"
	}
	return v.String()
}
func (p *TimeLocationPtr) Get() interface{} {
	return *(p.pp)
}
func NewTimeLocationVar(ppLocation **time.Location, name, usage string) {
	NewTimeLocationVarFlagSet(flag.CommandLine, ppLocation, name, usage)
}
func NewTimeLocationVarFlagSet(
		fs *flag.FlagSet, ppLocation **time.Location, name, usage string) {
	if ppLocation == nil {
		panic("pointer to *time.Location variable is nil!")
	}
	p := &TimeLocationPtr{pp: ppLocation}
	fs.Var(p, name, usage)
}

func UnixMillisToTime(timeMs int64) time.Time {
	return time.Unix(timeMs/1000, (timeMs%1000)*1000000)
}

func TimeToUnixMillis(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond()/1000000)
}

func MidnightOfSameDay(t time.Time) time.Time {
	y, m, d := t.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	return midnight
}

func MidnightOfNextDay(t time.Time) time.Time {
	y, m, d := t.Date()
	midnight := time.Date(y, m, d + 1, 0, 0, 0, 0, t.Location())
	return midnight
}

func SnapToHourOfDay(t time.Time, hour int) time.Time {
	y, m, d := t.Date()
	t2 := time.Date(y, m, d, hour, 0, 0, 0, t.Location())
	return t2
}

func FormatTimeOnly(t time.Time) (result string) {
	if t.Nanosecond() == 0 {
		if t.Second() == 0 {
			if t.Minute() == 0 {
				if t.Hour() == 0 {
					return "midnight"
				} else if t.Hour() == 12 {
					return "noon"
				}
				return t.Format("3pm")
			}
			return t.Format("3:04pm")
		}
		return t.Format("3:04:05pm")
	}
	return t.Format("3:04:05.999999pm")
}

// Not I18N. Not timezone aware.
func PrettyFutureTime(now, future time.Time) (result string) {
	if !future.After(now) || now.Location() != future.Location() {
		// Don't error out, just provide a hyper-detailed time.
		return future.Format(time.RFC3339Nano)
	}

	// TODO Handle DST changes (i.e. 23 and 25 hour days).

	midnight := MidnightOfNextDay(now)
	if future.Before(midnight) {
		return FormatTimeOnly(future)
	} else if future == midnight {
		return "midnight"
	}

	midnight = MidnightOfNextDay(midnight)
	if future.Before(midnight) {
		return "tomorrow at " + FormatTimeOnly(future)
	} else if future.Equal(midnight) {
		return "tomorrow at midnight"
	}

	for daysInFuture := 2; daysInFuture < 7; daysInFuture++ {
		midnight = MidnightOfNextDay(midnight)
		if future.Before(midnight) {
			return fmt.Sprintf("%s at %s", future.Weekday().String(),
												 FormatTimeOnly(future))
		} else if future.Equal(midnight) {
			return fmt.Sprintf("%s at midnight", future.Weekday().String())
		}
	}

	// I don't need any more at this point, but could add another loop
	// for the days of the following week (e.g. "next Tuesday at noon").

	return fmt.Sprintf("%s at %s", future.Format("2006-01-02"),
										 FormatTimeOnly(future))
}
