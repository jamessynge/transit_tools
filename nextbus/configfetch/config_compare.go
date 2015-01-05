package configfetch

// Support for comparing two config xml files (the result of
// FetchAgencyConfig, etc.) for equality (ignoring the differences
// caused by adding metadata to the xml files in the form of comments).

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/compare"
	"github.com/jamessynge/transit_tools/nextbus"
)

const (
	kCommentStart = "<!--"
	kCommentEnd   = "-->"
)

var commentStart, commentEnd []byte

func init() {
	commentStart = []byte(kCommentStart)
	commentEnd = []byte(kCommentEnd)
}

// Since the only difference between the Nextbus xml documents will usually
// be the comment that I add with metadata, find the first comment.
func findXmlComment(data []byte) (startIndex, endIndex int) {
	ndx1 := bytes.Index(data, commentStart)
	if ndx1 >= 0 {
		searchStart := ndx1 + len(commentStart)
		ndx2 := bytes.Index(data[searchStart:], commentEnd)
		if ndx2 >= 0 {
			return ndx1, searchStart + ndx2 + len(commentEnd)
		}
	}
	return -1, -1
}

func removeXmlComments(data []byte) []byte {
	searchStart := 0
	for {
		startIndex, endIndex := findXmlComment(data[searchStart:])
		if startIndex < 0 {
			return data
		}
		startIndex += searchStart
		endIndex += searchStart
		// Remove whitespace before and after too.
		for startIndex > 0 {
			c := data[startIndex-1]
			if c == ' ' || c == '\n' {
				startIndex--
			} else {
				break
			}
		}
		for endIndex < len(data) {
			c := data[endIndex]
			if c == ' ' || c == '\n' {
				endIndex++
			} else {
				break
			}
		}
		// Remove the comment and surrounding whitespace.
		glog.V(2).Infof("Removing comment: %q", data[startIndex:endIndex])
		copy(data[startIndex:], data[endIndex:])
		commentLength := endIndex - startIndex
		data = data[0 : len(data)-commentLength]
	}
}

// This is not general purpose comparison: we parse the file as if it contains
// nextbus's XML format before comparison, so if it contains other elements
// that are dropped, those won't be included in the comparison.
func compareTwoConfigFiles(dir1, dir2, rel string) (eq bool, err error) {
	fp1 := filepath.Join(dir1, rel)
	bytes1, err := ioutil.ReadFile(fp1)
	if err != nil {
		glog.Warningln("Error reading from", fp1, "\nError:", err)
		return false, err
	}
	fp2 := filepath.Join(dir2, rel)
	bytes2, err := ioutil.ReadFile(fp2)
	if err != nil {
		glog.Warningln("Error reading from", fp2, "\nError:", err)
		return false, err
	}
	if bytes.Equal(bytes1, bytes2) {
		glog.V(1).Infoln("Bytes are the same for", rel)
		return true, nil
	}
	if !strings.HasSuffix(rel, ".xml") {
		glog.Infoln("Non-xml files are different", rel)
		return false, nil
	}
	// Compare the non-comment bytes of the two xml files.
	nc1 := removeXmlComments(bytes1)
	nc2 := removeXmlComments(bytes2)
	if bytes.Equal(nc1, nc2) {
		glog.V(2).Infoln("Cleaned bytes are the same for", rel)
		return true, nil
	}
	body1, err := nextbus.UnmarshalNextbusXml(bytes1)
	if err != nil {
		glog.Warningln("Error parsing", fp1)
		return false, err
	}
	body2, err := nextbus.UnmarshalNextbusXml(bytes2)
	if err != nil {
		glog.Warningln("Error parsing", fp2)
		return false, err
	}
	eq, diffs := compare.DeepCompare(body1, body2)
	if !eq {
		glog.Infof("Files are different:\nFile 1: %s\nFile 2: %s", fp1, fp2)
		if !glog.V(1) && len(diffs) > 5 {
			diffs = diffs[0:5]
		}
		glog.Infof("Differences:\n%s", diffs)
	} else {
		glog.V(2).Infoln("Nextbus xml matches for", rel)
	}
	return eq, nil
}
