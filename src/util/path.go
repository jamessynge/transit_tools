package util

import (
	"fmt"
	"github.com/golang/glog"
	"os"
	"path/filepath"
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
