package nblocations

// Create a VehicleLocationsReport from a .xml, .html or .unknown file.

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
)

func FileToVlr(fp string) (*VehicleLocationsResponse, error) {
	if data, err := ioutil.ReadFile(fp); err == nil {
		var ft time.Time
		if stat, err := os.Stat(fp); err == nil {
			ft = stat.ModTime()
		}
		return DataToVlr(filepath.Base(fp), ft, data)
	} else {
		return nil, err
	}
}

const (
	// Milliseconds from Unix epoch; needs to start with non-zero (to simplify
	// code later that is measuring the length of the numeric string).
	kUnixTsPattern = `[1-9]\d+`
	// hhmmss or hhmmss.u+
	kTimeOfDayPattern = `\d{6}(?:\.\d+)`
	// yyyymmdd_hhmmss
	kDateAndTimePattern = `\d{8}_\d{6}`

	// Layout for parsing doesn't require fractional seconds; they're parsed
	// if present after seconds (expressed as 05 in the layout).
	kTimeOfDayLayout   = "150405"
	kDateAndTimeLayout = "20060102_150405"
)

var fnTimeRegexp *regexp.Regexp

func init() {
	pats := []string{
		kUnixTsPattern,
		kTimeOfDayPattern,
		kDateAndTimePattern,
	}
	pat := "^(" + strings.Join(pats, ")|(") + `)(?:$|\.)`
	fnTimeRegexp = regexp.MustCompile(pat)
}

// Initializes the RequestTime and ResponseTime of the VLR.
// May also return an error if unable to parse a time out of the file name.
// TODO Ensure location in ft is appropriate.
func addTimeToVlr(
	vlr *VehicleLocationsResponse, ft time.Time, fn string) (err error) {
	// If ft has a non-default TZ, use it, else use the local TZ.
	location := ft.Location()
	if location == time.UTC || strings.EqualFold("utc", location.String()) {
		location = time.Local
		if !ft.IsZero() {
			ft = ft.In(location)
		}
	}
	// Extract a timestamp and extension from the file name.
	var i int64
	var t time.Time
	matches := fnTimeRegexp.FindStringSubmatch(fn)
	if matches != nil {
		if len(matches[1]) > 10 {
			// A single number, which we'll treat as Unix epoch milliseconds.
			i, err = strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				t = util.UnixMillisToTime(i).In(location)
			}
		} else if len(matches[1]) == 10 {
			// A single number, which we'll treat as Unix epoch seconds.
			i, err = strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				t = time.Unix(i, 0).In(location)
			}
		} else if len(matches[2]) > 0 {
			t, err = time.ParseInLocation(kTimeOfDayLayout, matches[2], location)
		} else if len(matches[3]) > 0 {
			t, err = time.ParseInLocation(kDateAndTimePattern, matches[3], location)
		} else {
			// Programming error if matched, but no known group is set.
			panic("Did you add a new pattern to the regexp?")
		}
	}
	if err == nil && !t.IsZero() {
		if t.Year() == 0 {
			// We have an incomplete time. Is ft complete?
			if ft.IsZero() || ft.Year() == 0 {
				return fmt.Errorf("Unable to determine year from ft (%s) or fn (%s)",
					ft, fn)
			}
			// Yes, so we'll get the date from ft, and the time from t.
			hour := t.Hour()
			minute := t.Minute()
			second := t.Second()
			ns := t.Nanosecond()
			dayOffset := 0
			if ft.Hour() == 23 && hour == 0 {
				// At end of ft's day, but the end of t's day.
				dayOffset = 1
				t = time.Date(ft.Year(), ft.Month(), ft.Day()+1,
					hour, minute, second, ns, location)
			} else if hour == 23 && ft.Hour() == 0 {
				// At the beginning of ft's day, but the end of t's day.
				dayOffset = -1
			}
			t = time.Date(ft.Year(), ft.Month(), ft.Day()+dayOffset,
				hour, minute, second, ns, location)
		}
		ft = t
	}
	vlr.RequestTime = ft
	vlr.ResultTime = ft
	return
}

func hasXmlComment(data []byte) bool {
	start := bytes.IndexAny(data, "<!--")
	if start < 0 {
		return false
	}
	data = data[start+4:]
	return bytes.IndexAny(data, "-->") >= 0
}

// Takes the filename (fn) so that non-xml files can be declared
// (though we can still try to double check that apparently xml
// files really are).
func DataToVlr(fn string, ft time.Time, data []byte) (
	*VehicleLocationsResponse, error) {
	ext := filepath.Ext(fn)
	fn = filepath.Base(fn)
	vlr := &VehicleLocationsResponse{
		Body:     data,
		Response: &http.Response{},
	}
	errs := util.NewErrors()
	errs.AddError(addTimeToVlr(vlr, ft, fn))
	contentType := http.DetectContentType(data)
	if utf8.Valid(data) {
		contentType = contentType + "; charset=utf-8"
	}
	commented := hasXmlComment(data)
	if strings.HasPrefix(contentType, "text/xml") {
		errs.AddError(xmlToVlr(vlr))
	} else if strings.HasPrefix(contentType, "text/html") {
		errs.AddError(htmlToVlr(vlr))
	} else if commented {
		errs.AddError(htmlToVlr(vlr))
	} else {
		if ext != "unknown" {
			glog.Infof("Unexpected extension on filename: %s", fn)
		}
		errs.AddError(unknownToVlr(vlr))
	}
	return vlr, errs.ToError()
}

func parseCommentBytes(commentBytes []byte, vlr *VehicleLocationsResponse) (
	foundEntries bool, nonHeaders map[string]string) {
	foundEntries = false
	Trim := strings.TrimSpace
	lines := strings.Split(Trim(string(commentBytes)), "\n")
	for ndx, line := range lines {
		lines[ndx] = Trim(line)
	}
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "URL=") {
		// TODO Handle early cases where only the URL was added, and in the same
		// format as a header (i.e. "Name: Value", not "Name=Value").

		return
	}
	ndx := 0
	nonHeaders = make(map[string]string)
	for ; ndx < len(lines); ndx++ {
		line := lines[ndx]
		if len(line) == 0 {
			ndx++
			break
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) == 2 {
			nonHeaders[Trim(kv[0])] = Trim(kv[1])
			foundEntries = true
		}
	}

	for ; ndx < len(lines); ndx++ {
		if len(lines[ndx]) == 0 {
			continue
		}
		kv := strings.SplitN(lines[ndx], ":", 2)
		if len(kv) == 2 {
			vlr.Response.Header.Add(Trim(kv[0]), Trim(kv[1]))
			foundEntries = true
		}
	}
	return
}

func parseFirstValidCommentBytes(
	data []byte, commentsStartAndEnd []int, vlr *VehicleLocationsResponse) (
	foundEntries bool, nonHeaders map[string]string) {
	for ndx := 0; ndx+1 < len(commentsStartAndEnd); ndx += 2 {
		start := commentsStartAndEnd[ndx]
		end := commentsStartAndEnd[ndx+1]
		commentBytes := data[start:end]
		foundEntries, nonHeaders = parseCommentBytes(commentBytes, vlr)
		if foundEntries {
			return
		}
	}
	return false, nil
}

func copyString(dst *string, v string) {
	if len(v) > 0 {
		if len(*dst) > 0 && *dst != v {
			glog.Infof("Overwriting value %q with %q", *dst, v)
		}
		*dst = v
	}
}

func copyInt(dst *int, s string) {
	if len(s) > 0 {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			glog.Errorf("Unable to parse %q as integer: %s", s, err)
			return
		}
		if *dst != 0 && *dst != int(v) {
			glog.Infof("Overwriting value %d with %d", *dst, v)
		}
		*dst = int(v)
	}
}

var (
	kTimesRegexp = regexp.MustCompile(`^(\d+)(?: \((.*)\))?$`)
)

// Parse a time that consists of a Unix Milliseconds value, optionally followed
// by a formatted time value enclosed in parentheses.
// kArchiveTimeLayout is defined in vlr_archiver.go
func parseTimes(s string, doParseFormatted bool, location *time.Location) (
	msTime, fmtTime time.Time, err error) {
	matches := kTimesRegexp.FindStringSubmatch(s)
	if matches == nil {
		err = fmt.Errorf("Unable to match kTimesRegexp: %q", s)
		glog.Warning(err)
		return
	}
	var i int64
	i, err = strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		glog.Warningf("Unable to parse %q as an integer: %s", matches[1], err)
		return
	}
	if i <= 0 {
		return
	}
	msTime = util.UnixMillisToTime(i)
	if len(matches[2]) > 0 && doParseFormatted {
		// Not sure if I need the location, given that the layout
		// includes a TZ offset. In fact, will this have a bad effect?
		fmtTime, err = time.ParseInLocation(kArchiveTimeLayout, matches[2], location)
		if err != nil {
			glog.Warningf("Unable to parse %q as a formatted time: %s", matches[2], err)
			return
		}
	}
	return
}

func processNonHeaders(nonHeaders map[string]string,
	vlr *VehicleLocationsResponse) error {
	if len(nonHeaders) == 0 {
		return nil
	}
	errs := util.NewErrors()
	for k, v := range nonHeaders {
		switch k {
		case "URL":
			copyString(&vlr.Url, v)
		case "LastLastTime":
			ms, formatted, err := parseTimes(v, true, time.Local)
			errs.AddError(err)
			if ms.IsZero() {
				break
			}
			// Some files had the ms value correct, but not the formatted value,
			// which instead was the ResultTime.
			if !formatted.IsZero() && !ms.Equal(formatted) {
				glog.Infof("Found mismatched LastLastTime\nm: %s\nf: %s", ms, formatted)
			}
			vlr.LastLastTime = ms
		case "LastTime":
			ms, _, err := parseTimes(v, false, time.Local)
			errs.AddError(err)
			if !ms.IsZero() {
				vlr.LastTime = ms
			}
		case "RequestTime":
			fmtTime, err := time.ParseInLocation(kArchiveTimeLayout, v, time.Local)
			errs.AddError(err)
			if !fmtTime.IsZero() {
				vlr.RequestTime = fmtTime
				// TODO Compare with timee already in vlr.RequestTime?
			}
		case "ResultTime":
			fmtTime, err := time.ParseInLocation(kArchiveTimeLayout, v, time.Local)
			errs.AddError(err)
			if !fmtTime.IsZero() {
				vlr.ResultTime = fmtTime
			}
		case "Status":
			copyString(&vlr.Response.Status, v)
		case "StatusCode":
			copyInt(&vlr.Response.StatusCode, v)
		case "Error":
			t, err := strconv.Unquote(v)
			if err != nil {
				errs.AddError(err)
				glog.Errorf("Unable to parse Error value as quoted string: %s", v)
			} else {
				err = errors.New(t)
				if vlr.Error != nil {
					glog.Infof("Overwriting Error\nOld: %s\nNew: %s", vlr.Error, err)
				}
				vlr.Error = err
			}
		default:
			glog.Warningf("Unknown non-header: %s=%s", k, v)
		}
	}
	return errs.ToError()
}

func xmlToVlr(vlr *VehicleLocationsResponse) error {
	errs := util.NewErrors()
	bodyElem, err := nextbus.UnmarshalVehicleLocationsBytes(vlr.Body)
	errs.AddError(err)
	if bodyElem != nil {
		copyString(&vlr.Url, bodyElem.RequestUrl)
		if len(bodyElem.RequestTimeMs) > 0 {
			i, err := strconv.ParseInt(bodyElem.RequestTimeMs, 10, 64)
			if err != nil {
				errs.AddError(err)
			} else {
				t := util.UnixMillisToTime(i)
				// TODO Add warning if t is very different from vlr.RequestTime
				vlr.RequestTime = t
				vlr.ResultTime = t
			}
		}
		vlr.Report, err = nextbus.ConvertVehicleLocationsBodyToReport(bodyElem)
		errs.AddError(err)
	}
	errs.AddError(htmlToVlr(vlr))
	return errs.ToError()
}

func htmlToVlr(vlr *VehicleLocationsResponse) error {
	errs := util.NewErrors()
	commentsStartAndEnd, err := util.FindCommentsInXml(vlr.Body)
	errs.AddError(err)
	_, nonHeaders := parseFirstValidCommentBytes(
		vlr.Body, commentsStartAndEnd, vlr)
	errs.AddError(processNonHeaders(nonHeaders, vlr))
	return errs.ToError()
}

var (
	kUrlEq   = []byte("URL=")
	kNNUrlEq = []byte("\n\nURL=")
)

func unknownToVlr(vlr *VehicleLocationsResponse) error {
	var offset int
	if bytes.HasPrefix(vlr.Body, kUrlEq) {
		offset = 0
	} else {
		offset = bytes.LastIndex(vlr.Body, kNNUrlEq)
		if offset < 0 {
			return fmt.Errorf("Found no comments with metadata")
		}
		offset += 2
	}
	_, nonHeaders := parseCommentBytes(vlr.Body[offset:], vlr)
	return processNonHeaders(nonHeaders, vlr)
}

func recreateHttpResponse(data []byte, headerText []byte) *http.Response {

	//	http.Response
	return nil

}
