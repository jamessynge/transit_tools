package configfetch

// Support for comparing two config dirs (the result of FetchAgencyConfig, etc.)
// for equality (ignoring the differences caused by adding metadata to the xml
// files in the form of comments).

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/util"
)

var compareStoppedErr error = errors.New("Config comparison stopped")
var CompareStoppedErr error = compareStoppedErr

func getDirDescendants(dir string, doStop *int32) ([]string, error) {
	errs := util.NewErrors()
	descendants := []string{}
	walkFn := func(path string, info os.FileInfo, err error) error {
		if atomic.LoadInt32(doStop) != 0 {
			return compareStoppedErr
		}
		if err != nil {
			errs.AddError(err)
			return nil
		}
		if !info.IsDir() {
			if rel, err := filepath.Rel(dir, path); err == nil {
				descendants = append(descendants, rel)
			} else {
				errs.AddError(err)
			}
		}
		return nil
	}
	err := filepath.Walk(dir, walkFn)
	if err != nil {
		return nil, err
	}
	util.SortPaths(descendants)
	return descendants, errs.ToError()
}

type DirectorySimilarity int

const (
	DirectoriesDifferent DirectorySimilarity = iota
	DirectoriesEquivalent
	Dir1MoreComplete
	Dir2MoreComplete
)

func partitionDirectoryEntrySets(entries1, entries2 []string) (
	commonEntries, onlyEntries1, onlyEntries2 []string) {
	ndx1, ndx2 := 0, 0
	for ndx1 < len(entries1) && ndx2 < len(entries2) {
		entry1, entry2 := entries1[ndx1], entries2[ndx2]
		if util.PathLess(entry1, entry2) {
			// There is an extra entry in entries1.
			onlyEntries1 = append(onlyEntries1, entry1)
			glog.Infoln("Extra dir1 file:", entry1)
			ndx1++
		} else if util.PathLess(entry2, entry1) {
			// There is an extra entry in entries2
			onlyEntries2 = append(onlyEntries2, entry2)
			glog.Infoln("Extra dir2 file:", entry2)
			ndx2++
		} else {
			// Both directories have a file with the same name.
			commonEntries = append(commonEntries, entry1)
			ndx1++
			ndx2++
		}
	}
	if ndx1 < len(entries1) {
		glog.Infof("There are %d extra entries at the end of entries1", len(entries1)-ndx1)
		onlyEntries1 = append(onlyEntries1, entries1[ndx1:]...)
	}
	if ndx2 < len(entries2) {
		glog.Infof("There are %d extra entries at the end of entries2", len(entries2)-ndx2)
		onlyEntries2 = append(onlyEntries2, entries2[ndx2:]...)
	}
	return
}

// Compare two config directories, returning a DirectorySimilarity value,
// which enables a caller to decide whether to cull dir1, dir2, or neither.
// Directories are different if they both contain a file with the same name
// (same relative path, really) and that file is semantically different.
func StoppableConfigDirsComparison(dir1, dir2 string, doStop *int32) (
	similarity DirectorySimilarity, err error) {
	entries1, err := getDirDescendants(dir1, doStop)
	if err != nil {
		return
	}
	glog.V(0).Infoln("Found", len(entries1), "entries in", dir1)
	entries2, err := getDirDescendants(dir2, doStop)
	if err != nil {
		return
	}
	glog.V(0).Infoln("Found", len(entries2), "entries in", dir2)
	// Compare the entry sets, before comparing the contents.
	commonEntries, onlyEntries1, onlyEntries2 := partitionDirectoryEntrySets(
		entries1, entries2)
	if len(onlyEntries1) > 0 && len(onlyEntries2) > 0 {
		glog.Infoln("Each has at least one unique file")
		similarity = DirectoriesDifferent
		return
	}
	glog.Info("Comparing files in common")
	// Compare the files they have in common.
	numFilesDifferent := 0
	for _, relpath := range commonEntries {
		if atomic.LoadInt32(doStop) != 0 {
			err = compareStoppedErr
			return
		}
		// Both directories have a file with the same name; are the files the same?
		// I'm assuming that the files are small enough that I don't need to
		// push doStop all the way into compareTwoConfigFiles().
		if eq, err := compareTwoConfigFiles(dir1, dir2, relpath); !eq || err != nil {
			numFilesDifferent++
			if numFilesDifferent >= 10 {
				glog.Infof("At least %d files are different", numFilesDifferent)
				return DirectoriesDifferent, nil
			}
		}
	}
	if numFilesDifferent > 0 {
		glog.Infof("%d files are different", numFilesDifferent)
		similarity = DirectoriesDifferent
		return
	}
	// Where both directories have a file with the same name, the files are
	// the same.
	if len(onlyEntries1) > 0 {
		// There are extra entries in entries1, and not in entries2.
		glog.Info("Dir1MoreComplete")
		similarity = Dir1MoreComplete
	} else if len(onlyEntries2) > 0 {
		// There are extra entries in entries2, and not in entries1.
		glog.Info("Dir2MoreComplete")
		similarity = Dir2MoreComplete
	} else {
		glog.Info("DirectoriesEquivalent")
		similarity = DirectoriesEquivalent
	}
	return
}

// Relatively trivial comparison, just returns true if the xml documents are
// all the same, false if they aren't, including if there are different
// numbers of files. Any error is deemed to indicate a difference, so we
// only delete config dirs when there is no difference.
func CompareConfigDirs(dir1, dir2 string) (similarity DirectorySimilarity, err error) {
	doStop := new(int32)
	*doStop = 0
	return StoppableConfigDirsComparison(dir1, dir2, doStop)
}
