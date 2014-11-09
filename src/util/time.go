package util
/*
On MS Windows, to track how well system time is tracking an NTP server, try
one of the following:
w32tm /stripchart /period:900 /computer:2.pool.ntp.org
w32tm /stripchart /period:900 /computer:time.nist.gov
*/
import (
	"fmt"
	"time"
)

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
