package configfetch

// Support for getting rid of duplicate config directories.  In particular,
// if we see 3 in a row that are the same, we eliminate the middle one.

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/util"
)

type ConfigCuller interface {
	// Add another config dir; directories should be added in ascending date order,
	// and only for a single agency.
	AddDir(newDir string)

	// Signal culler to stop immediately, but don't wait for that to happen;
	// instead just returns a channel on which the culler will signal that it
	// has stopped.
	RequestStop() chan bool

	// Signal culler to stop immediately, and wait for it to do so.
	Stop()

	// Signal culler to stop between config dir comparisons (i.e. don't interrupt).
	// Returns a channel on which the culler will signal that it has stopped.
	StopWhenReady() chan bool
}

type configCuller struct {
	// First and last directories of a sequence that are all the same.
	firstDir, prevDir string
	doStop            int32

	stopCh chan chan bool
	recvCh chan string

	// If true, then culler identifies candidates to remove but does not actually
	// delete them.
	compareOnly bool

	// If set, culler writes paths of directories selected for removal to this
	// channel.
	culledDirCh chan string
}

// Remove the dir, trying twice to accomodate an issue seen on Windows.
func rmdir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		// Sometimes on Windows I get an error, where all of the files and
		// directories in dir are removed, but not dir itself.
		if err2 := os.Remove(dir); err2 != nil {
			if util.Exists(dir) {
				return err
			}
		}
	}
	return nil
}

var deletionStoppedErr error = errors.New("Config deletion stopped")

func doRemoveConfigDir(configDir string, doStop *int32) {
	// Create an entry marking this as being deleted.  Using a directory
	// rather than a file so that the following deletion code will delete it
	// at the end.
	marker := filepath.Join(configDir, "DELETING_CONFIG_DIR")
	if err := os.Mkdir(marker, 0400); err != nil {
		glog.Warningln("Unable to create marker directory", marker,
			"\nError:", err)
	}
	// Delete just the files in the directory on the first pass, as these will
	// take longer than deleting the empty directories left after that.
	encounteredError := false
	var directories []string
	walkFn := func(fp string, info os.FileInfo, err error) error {
		if atomic.LoadInt32(doStop) != 0 {
			return deletionStoppedErr
		}
		if err != nil {
			glog.Warningln("Error while lstat-ing", fp, "\nError:", err)
			encounteredError = true
		} else if info.IsDir() {
			directories = append(directories, fp)
		} else {
			glog.V(2).Infoln("Removing", fp)
			err := os.Remove(fp)
			if err != nil {
				glog.Warningf("Error from os.Remove(%s)\nError: %s", fp, err)
				encounteredError = true
			}
		}
		return nil
	}
	if err := filepath.Walk(configDir, walkFn); err != nil {
		if err == deletionStoppedErr {
			glog.Info("Stopped deletion of", configDir)
		} else {
			glog.Warningf("Error walking config dir %s\nError: %s",
				configDir, err)
		}
		return
	}
	if encounteredError {
		return
	}
	// Now delete all the directories, in reverse order from the order in which
	// they were found (configDir will be the first entry in directories).
	for i := len(directories) - 1; i >= 0; i-- {
		if atomic.LoadInt32(doStop) != 0 {
			glog.Info("Stopped deletion of", configDir)
			return
		}
		dir := directories[i]
		if err := rmdir(dir); err != nil {
			glog.Warningf("Error trying to Remove %s\nError: %s", dir, err)
			encounteredError = true
		}
	}
	if encounteredError {
		return
	}
	// Remove parent if empty (e.g. directory for a day, which normally
	// might have several directories in it, unless they are all identical).
	// No need to bother with clearing out at the month level; it is less likely
	// that we'll have a month with no changes, and I don't want to risk writing
	// a recursive algorithm here that might wipe the entire disk!
	parent := filepath.Dir(configDir)
	if util.IsEmptyDirectory(parent, false) {
		err := os.Remove(parent)
		if err != nil {
			glog.Warningf("Error deleting parent directory %s\nError: %s",
				parent, err)
			return
		}
	}
}

func (p *configCuller) cullDir(dir string) {
	if p.compareOnly {
		glog.Infoln("Would remove config dir:", dir)
	} else {
		glog.Infoln("Removing config dir:", dir)
		doRemoveConfigDir(dir, &p.doStop)
	}
	if p.culledDirCh != nil {
		// Send the path, but don't wait for it to be sent.
		go func(d string) { p.culledDirCh <- d }(dir)
	}
}

func (p *configCuller) processCurrentDir(currentDir string) {
	glog.Infof(`Starting config dir comparison:
previous: %s
 current: %s`, p.prevDir, currentDir)
	similarity, err := StoppableConfigDirsComparison(p.prevDir, currentDir, &p.doStop)
	if err == compareStoppedErr {
		glog.Info("Config dir comparison stopped")
		// We expect to receive stoppedCh soon, make sure we don't resume
		// (easily).
		p.firstDir = ""
		p.prevDir = ""
		return
	}

	if err != nil {
		glog.Errorf(`Errors while comparing config dirs:
previous: %s
 current: %s
   error: %s`, p.prevDir, currentDir, err)
		// Since we don't know if the error is with the prevDir or currentDir,
		// keep them both for later analysis.
		p.firstDir = ""
		p.prevDir = ""
		return
	}

	if similarity == DirectoriesEquivalent {
		// Most common case.
		glog.Infoln("Configurations are the same")
		if p.firstDir == p.prevDir {
			glog.Info("firstDir and prevDir are the same, no middle dir yet")
		} else {
			glog.Infoln("Duplicate middle dir:", p.prevDir)
			p.cullDir(p.prevDir)
		}
		p.prevDir = currentDir
		return
	}

	if similarity == DirectoriesDifferent {
		glog.Infoln("Configurations are different")
		p.firstDir = currentDir
		p.prevDir = currentDir
		return
	}

	if similarity == Dir1MoreComplete {
		glog.Infoln("previous dir is more complete than current dir")
		p.cullDir(currentDir)
		return
	}

	if similarity == Dir2MoreComplete {
		// Unlikely, but possible.
		if p.firstDir == p.prevDir {
			glog.Infoln("current dir is more complete than previous dir")
			p.cullDir(p.prevDir)
		} else {
			glog.Infoln("current dir is more complete than first and previous dirs")
			p.cullDir(p.firstDir)
			p.cullDir(p.prevDir)
		}
		p.firstDir = currentDir
		p.prevDir = currentDir
		return
	}

	glog.Errorln("Unknown similarity value:", similarity)
}

func runCuller(p *configCuller) {
	for {
		select {
		case stoppedCh := <-p.stopCh:
			// Need to stop now.
			stoppedCh <- true
			if p.culledDirCh != nil {
				close(p.culledDirCh)
				p.culledDirCh = nil
			}
			return

		case currentDir := <-p.recvCh:
			if p.prevDir == "" {
				// Startup case.
				glog.V(1).Info("prevDir is empty")
				p.firstDir = currentDir
				p.prevDir = currentDir
			} else {
				p.processCurrentDir(currentDir)
			}
		}
	}
}

func (p *configCuller) AddDir(newDir string) {
	p.recvCh <- newDir
}

// Signal culler to stop immediately, but don't wait for that to happen;
// instead just returns a channel on which the culler will signal that it
// has stopped.
func (p *configCuller) RequestStop() chan bool {
	glog.Info("Requesting config culler to stop")
	atomic.StoreInt32(&p.doStop, 1)
	stoppedCh := make(chan bool)
	p.stopCh <- stoppedCh
	return stoppedCh
}

// Signal culler to stop immediately, and wait for it to do so.
func (p *configCuller) Stop() {
	ch := p.RequestStop()
	<-ch
	glog.Info("Config culler stopped")
}

// Signal culler to stop between config dir comparisons (i.e. don't interrupt).
func (p *configCuller) StopWhenReady() chan bool {
	stoppedCh := make(chan bool)
	p.stopCh <- stoppedCh
	return stoppedCh
}

func NewConfigCuller() ConfigCuller {
	state := &configCuller{
		stopCh: make(chan chan bool),
		recvCh: make(chan string),
	}
	go runCuller(state)
	return state
}

func NewConfigCuller2(compareOnly bool, culledDirCh chan string) ConfigCuller {
	state := &configCuller{
		stopCh:      make(chan chan bool),
		recvCh:      make(chan string),
		compareOnly: compareOnly,
		culledDirCh: culledDirCh,
	}
	go runCuller(state)
	return state
}
