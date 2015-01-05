package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/golang/glog"
)

func ChooseUniqueName(dir, base, ext string) (string, error) {
	path := filepath.Join(dir, base+ext)
	if !Exists(path) {
		return path, nil
	}
	// Choose a path that doesn't exist.
	for ndx := 1; ndx < 1000; ndx++ {
		path = filepath.Join(dir, fmt.Sprintf("%s_%03d%s", base, ndx, ext))
		if !Exists(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("Unable to produce unique filename starting with: %s",
		filepath.Join(dir, base+ext))
}

func OpenUniqueFile(dir, base, ext string, dirMode, fileMode os.FileMode) (
	f *os.File, path string, err error) {
	if err = os.MkdirAll(dir, dirMode); err != nil {
		err = fmt.Errorf("Unable to create directory: %s\nError: %v", dir, err)
		return
	}
	const flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND | os.O_EXCL
	firstPath := filepath.Join(dir, base+ext)
	path = firstPath
	ndx := 0
	for {
		f, err = os.OpenFile(path, flag, fileMode)
		if err == nil {
			return
		}

		// DEBUG
		glog.V(1).Infof("Error creating file: %s\nError: %v", path, err)

		ndx++
		if ndx >= 1000 {
			err = fmt.Errorf("Unable to create a unique file starting with: %s",
				firstPath)
			return
		}
		path = filepath.Join(dir, fmt.Sprintf("%s_%03d%s", base, ndx, ext))
	}
}

func Exists(name string) bool {
	fi, err := os.Stat(name)
	//	log.Printf("Exists: err=%v     fi=%v", err, fi)
	return err == nil && fi != nil
}

func IsFile(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi != nil && !fi.IsDir()
}

func IsDirectory(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi != nil && fi.IsDir()
}

// Is path_a modified more recently than path_b?
func IsNewer(path_a, path_b string) bool {
	stat_a, err := os.Stat(path_a)
	if err != nil {
		return false
	}
	stat_b, err := os.Stat(path_b)
	if err == nil && stat_a.ModTime().After(stat_b.ModTime()) {
		glog.V(2).Infof("%s modified at %s", path_a, stat_a.ModTime().String())
		glog.V(2).Infof("%s modified at %s", path_b, stat_b.ModTime().String())
		return true
	}
	return false
}

func IsEmptyDirectoryOrError(s string) (bool, error) {
	//	fi, err := os.Stat(s)
	//	if err != nil {
	//		return false, err
	//	}
	//	if !fi.IsDir() {
	//		return false, nil
	//	}
	f, err := os.Open(s)
	if err != nil {
		return false, err
	}
	defer f.Close()
	names, err := f.Readdirnames(10)
	if err == io.EOF && len(names) == 0 {
		return true, nil
	}
	return false, err
}

func IsEmptyDirectory(s string, defaultIfUnsure bool) bool {
	result, err := IsEmptyDirectoryOrError(s)
	if err != nil {
		return defaultIfUnsure
	}
	return result
}

func ExpandPathGlobs(paths, sep string) (expansion []string, err error) {
	if sep == "" {
		sep = string(os.PathListSeparator)
	}
	errs := NewErrors()
	for _, glob := range strings.Split(paths, sep) {
		glog.V(1).Infof("Expanding %q", glob)
		glob = strings.TrimSpace(glob)
		if glob == "" {
			continue
		}
		matches, err := filepath.Glob(glob)
		if err != nil {
			glog.Errorln("Error expanding glob", glob, "\nError:", err)
			errs.AddError(err)
			continue
		}
		glog.V(1).Infof("Produced %d paths", len(matches))
		expansion = append(expansion, matches...)
	}
	err = errs.ToError()
	return
}

const (
	osPathSeparatorString = string(os.PathSeparator)
	pathSeparators        = "/" + string(os.PathSeparator)
)

func SplitPath(s string) (result []string) {
	if len(s) == 0 {
		return
	}
	if os.PathSeparator == '/' {
		// Can only contain / as a separator (i.e. no other rune allowed).
		return strings.Split(s, "/")
	}
	hasOPS := strings.ContainsRune(s, os.PathSeparator)
	hasSlash := strings.Contains(s, "/")
	if !hasOPS {
		return strings.Split(s, "/")
	}
	if !hasSlash {
		return strings.Split(s, osPathSeparatorString)
	}
	// Has both / and the OS path separator, which I assume are all
	// intended as separators (on Windows '/' and '\' are both acceptable
	// between a dir and an entry in that dir, including another dir).
	b := []byte(s)
	for {
		n := bytes.IndexAny(b, pathSeparators)
		if n < 0 {
			// No more separators.
			result = append(result, string(b))
			return
		}
		result = append(result, string(b[0:n]))
		b = b[n:]
		_, l := utf8.DecodeRune(b)
		b = b[l:]
	}
}

func PathPartsLess(parts1, parts2 []string) bool {
	for n := 0; n < len(parts1) && n < len(parts2); n++ {
		if parts1[n] == parts2[n] {
			continue
		}
		if n+1 == len(parts1) && n+1 < len(parts2) {
			// Within a directory, what files to sort before directories.
			return true
		}
		if n+1 < len(parts1) && n+1 == len(parts2) {
			// Within a directory, what files to sort before directories.
			return false
		}
		return parts1[n] < parts2[n]
	}
	if len(parts1) == len(parts2) {
		return false
	} // Paths are equal.
	return len(parts1) < len(parts2)
}

func PathLess(path1, path2 string) bool {
	if path1 == path2 {
		return false
	}
	parts1, parts2 := SplitPath(path1), SplitPath(path2)
	return PathPartsLess(parts1, parts2)
}

// Sort a slice of paths, considering each element of the path separately.
// a/a sorts after a/, which sorts after a.
func SortPaths(paths []string) {
	parts := make(map[string][]string)
	for _, p := range paths {
		parts[p] = SplitPath(p)
	}
	less := func(i, j int) bool {
		pi, pj := parts[paths[i]], parts[paths[j]]
		return PathPartsLess(pi, pj)
	}
	less2 := func(i, j int) bool {
		v := less(i, j)
		if v {
			glog.Infof("Path i (%d) is less than path j (%d):\ni: %s\nj: %s",
				i, j, paths[i], paths[j])
		}
		return v
	}
	swap := func(i, j int) {
		//		glog.Infof("Swapping paths at i (%d) and j (%d):\ni: %s\nj: %s",
		//							 i, j, paths[i], paths[j])
		paths[i], paths[j] = paths[j], paths[i]
	}
	if glog.V(2) {
		Sort3(len(paths), less2, swap)
	} else {
		Sort3(len(paths), less, swap)
	}

	if glog.V(1) {
		glog.Info("Sorted path entries:")
		for _, p := range paths {
			glog.Info("\t", p)
		}
	}
}
