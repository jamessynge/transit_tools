package nblocations

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"util"
)

// Archives the XML responses from NextBus as files in a tar.
type VLRArchiver struct {
	dta *util.DatedTarArchiver

	// Layout for time.Time.Format which produces the base of the tar entry
	// file name given the date of the report. If unspecified (empty string),
	// defaults to:
	//      "20060102_150405"
	// Producing names that are unique down to the second.
	FileNameBaseLayout string
}

func NewVLRArchiver(dta *util.DatedTarArchiver) *VLRArchiver {
	return &VLRArchiver{dta:dta, FileNameBaseLayout: "20060102_150405"}
}

func (p *VLRArchiver) PartialFlush() error {
	return p.dta.PartialFlush()
}

func (p *VLRArchiver) Flush() error {
	return p.dta.Flush()
}

func (p *VLRArchiver) Close() error {
	return p.dta.Close()
}

var xmlCommentCleaner = strings.NewReplacer("-->", "-%2D>")

var standardComments = map[string]string{
	"Access-Control-Allow-Origin": "*",
	"Connection":                  "Keep-Alive",
	"Content-Type":                "text/xml",
	"Keep-Alive":                  "timeout=5, max=100",
	"Vary":                        "Accept-Encoding",
	"X-Frame-Options":             "SAMEORIGIN",
}
func isStandardComment(key, value string) bool {
	if v, ok := standardComments[key]; ok {
		return v == value
	}
	return false
}

// Create the body of a comment which can be added to the archive of a fetch.
func makeArchiveComment(vlr *VehicleLocationsResponse, cleanForXml bool) []byte {
	skipStandardComments := true
	var b bytes.Buffer
	b.Grow(2048)
	b.WriteString(fmt.Sprintf("URL=%s\n", vlr.Url))
	timeLayout := "2006-01-02T15:04:05.999Z07:00"
	llt := util.TimeToUnixMillis(vlr.LastLastTime)
	if llt > 0 {
		b.WriteString(fmt.Sprintf("LastLastTime=%d (%s)\n",
			llt, vlr.RequestTime.Format(timeLayout)))
	} else {
		b.WriteString(fmt.Sprintf("LastLastTime=%d\n", llt))
	}
	b.WriteString(fmt.Sprintf("RequestTime=%s\n",
		vlr.RequestTime.Format(timeLayout)))
	b.WriteString(fmt.Sprintf("ResultTime=%s\n",
		vlr.ResultTime.Format(timeLayout)))
	if vlr.Response != nil && vlr.Response.StatusCode != http.StatusOK {
		b.WriteString(fmt.Sprintf("Status=%s\n", vlr.Response.Status))
		b.WriteString(fmt.Sprintf("StatusCode=%d\n", vlr.Response.StatusCode))
		skipStandardComments = false
	}
	if vlr.Error != nil {
		b.WriteString(fmt.Sprintf("Error=%q\n", vlr.Error))
		skipStandardComments = false
	}
	if vlr.Response != nil {
		b.WriteString("\n")
		for _, kvs := range util.SortHeaderItems(vlr.Response.Header) {
			key := kvs.Key
			for _, value := range kvs.Values {
				if skipStandardComments && isStandardComment(key, value) {
					continue
				}
				b.WriteString(key)
				b.WriteString(": ")
				b.WriteString(value)
				b.WriteString("\n")
			}
		}
	}
	if cleanForXml {
		s := b.String()
		s = xmlCommentCleaner.Replace(s)
		return []byte(s)
	}
	return b.Bytes()
}

func (p *VLRArchiver) AddResponse(vlr *VehicleLocationsResponse) (err error) {
  var ext, filename string
	surroundComment := true
	lengthBeforeComments := 0
	resumeBodyAt := 0
	var parts [][]byte

	if BodyIsXml(vlr) {
		ext = ".xml"
		lengthBeforeComments, resumeBodyAt, err = util.FindRootXmlElementOffset(vlr.Body)
	} else if BodyIsHtml(vlr) {
		ext = ".html"
		lengthBeforeComments, resumeBodyAt, err = util.FindRootXmlElementOffset(vlr.Body)
	}

	bodyLen := len(vlr.Body)
	if err != nil || ext == "" {
		surroundComment = false
		ext = ".unknown"
		lengthBeforeComments = bodyLen
	}

	// Ensure everything is in range.
	if lengthBeforeComments < 0 {
		lengthBeforeComments = 0
	} else if bodyLen < lengthBeforeComments {
		lengthBeforeComments = bodyLen
	}
	if resumeBodyAt < lengthBeforeComments {
		resumeBodyAt = lengthBeforeComments
	} else if bodyLen < resumeBodyAt {
		resumeBodyAt = bodyLen
	}

	commentBytes := makeArchiveComment(vlr, surroundComment)

	// Make the contents of the file: a list of byte slices, including additional
	// information in commentBytes.
	if lengthBeforeComments > 0 {
		parts = append(parts, vlr.Body[0:lengthBeforeComments])
	}

	if len(commentBytes) > 0 {
		if surroundComment {
			parts = append(parts, []byte("\n<!--\n"))
		} else if lengthBeforeComments > 0 {
			parts = append(parts, []byte("\n\n"))
		}
		parts = append(parts, commentBytes)
		if surroundComment {
			parts = append(parts, []byte("-->\n"))
		} else if resumeBodyAt < bodyLen {
			parts = append(parts, []byte("\n\n"))
		}
	}

	if resumeBodyAt < bodyLen {
		parts = append(parts, vlr.Body[resumeBodyAt:])
	}

	// On the basis of the experience that the time reported by the NextBus
	// servers goes backwards most days around 2am (e.g. when they sync to a
	// clock standard), let's not use that as the basis for our filename.
	ts := vlr.RequestTime
	if ts.Before(vlr.ResultTime) {
		// Half way between when we make the request, and when we got the response.
		ts = ts.Add(vlr.ResultTime.Sub(ts) / 2)
	}
	filename = ts.Format(p.FileNameBaseLayout) + ext

	return p.dta.AddFileParts(ts, filename, parts)
}
