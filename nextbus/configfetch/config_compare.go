package configfetch

import (
"bytes"
"io/ioutil"
"path/filepath"
"strings"
"os"

"github.com/golang/glog"

"github.com/jamessynge/transit_tools/compare"
"github.com/jamessynge/transit_tools/nextbus"
"github.com/jamessynge/transit_tools/util"
)

func getDirDescendants(dir string) ([]string, error) {
	errs := util.NewErrors()
  descendants := []string{}
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err == nil {
			rel, err := filepath.Rel(dir, path)
			if err == nil {
				if !info.IsDir() {
					descendants = append(descendants, rel)
				}
			} else {
				errs.AddError(err)
			}
		} else {
			errs.AddError(err)
		}
		return nil
  }
  err := filepath.Walk(dir, walkFn)
  if err != nil {
  	return nil, err
  }
  return descendants, errs.ToError()
}

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

func removeXmlComments(data[] byte) []byte {
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
			c := data[startIndex - 1]
			if c == ' ' || c == '\n' {
				startIndex--
			} else {
				break
			}
		}
		for endIndex < len(data) {
			c := data[endIndex]
			if c == ' ' || c == '\n' {
				endIndex ++
			} else {
				break
			}
		}
		// Remove the comment and surrounding whitespace.
		glog.V(2).Infof("Removing comment: %q", data[startIndex:endIndex])
		copy(data[startIndex:], data[endIndex:])
		commentLength := endIndex - startIndex
		data = data[0:len(data) - commentLength]
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
		glog.V(1).Infoln("Non-xml files are different", rel)
		return false, nil
	}
	bytes1 = removeXmlComments(bytes1)
	bytes2 = removeXmlComments(bytes2)
	if bytes.Equal(bytes1, bytes2) {
		glog.V(1).Infoln("Cleaned bytes are the same for", rel)
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
		glog.Infof("Files are different:\nFile 1: %s\nFile 2: %s\nDiffs:\n%s",
							 fp1, fp2, diffs)
	} else {
		glog.V(1).Infoln("Nextbus xml matches for", rel)
	}
	return eq, nil
}

// Relatively trivial comparison, just returns true if the xml documents are
// all the same, false if they aren't, including if there are different
// numbers of files. Any error is deemed to indicate a difference, so we
// only delete config dirs when there is no difference.
func CompareConfigDirs(dir1, dir2 string) (eq bool, err error) {
	entries1, err := getDirDescendants(dir1)
	if err != nil {
		return false, err
	}
	glog.Infoln("Found", len(entries1), "entries in", dir1)
	entries2, err := getDirDescendants(dir2)
	if err != nil {
		return false, err
	}
	glog.Infoln("Found", len(entries2), "entries in", dir2)
	if len(entries1) != len(entries2) {
		// Clearly different somewhere.
		return false, nil
	}
	for ndx, _ := range entries1 {
		entry1, entry2 := entries1[ndx], entries2[ndx]
		if entry1 < entry2 {
			// There is an extra entry in entries1.
			return false, nil
		}
		if entry1 > entry2 {
			// There is an extra entry in entries2
			return false, nil
		}
		// Are the files the same?
		if eq, err := compareTwoConfigFiles(dir1, dir2, entry1); !eq || err != nil {
			return false, err
		}
	}
	return true, nil
}
