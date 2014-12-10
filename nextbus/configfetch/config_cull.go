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

type ConfigCuller struct {
	// First and last directories of a sequence that are all the same.
	firstDir, prevDir string
	doStop int32

	stopCh chan chan bool
	recvCh chan string
}

var deletionStoppedErr error = errors.New("Config deletion stopped")

func doRemoveConfigDir(configDir string, doStop *int32) {
	// Create an entry marking this as being deleted.  Using a directory
	// rather than a file so that the following deletion code will delete it
	// at the end.
	marker := filepath.Join(configDir, "DELETING_IDENTICAL_CONFIG_DIR")
	if err := os.Mkdir(marker, 0400); err != nil {
		glog.Warningln("Unable to create marker directory", marker,
									 "\nError:", err)
	}
	// Delete just the files in the directory on the first pass, as these will
	// take longer than deleting the empty directories left after that.
	encounteredError := false
	walkFn := func(fp string, info os.FileInfo, err error) error {
		if atomic.LoadInt32(doStop) != 0 {
			return deletionStoppedErr
		}
		if err != nil {
			glog.Warningln("Error while lstat-ing", fp, "\nError:", err)
			encounteredError = true
		} else if !info.IsDir() {
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
			glog.Warningf("Error deleting contents of config dir %s\nError: %s",
									  configDir, err)
		}
		return
	}
	if encounteredError {
		return
	}
	// Now delete all the directories.
	if err := os.RemoveAll(configDir); err != nil {
		glog.Warningf("Error from os.RemoveAll(%s)\nError: %s", configDir, err)
		return
	}
	// Remove parent if empty (e.g. directory for a day, which normally
	// might have several directories in it, unless they are all identical).
	// No need to bother with clearing out at the month level; it less likely
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

func runCuller(p *ConfigCuller) {
	for {
		select {
		case stoppedCh := <- p.stopCh:
			// Need to stop now.
			stoppedCh <- true
			return

		case currentDir := <- p.recvCh:
			if p.prevDir == "" {
				// Startup case.
				glog.V(1).Info("prevDir is empty")
				p.firstDir = currentDir
				p.prevDir = currentDir
				continue
			}
			glog.Infof(`Starting config dir comparison:
previous: %s
 current: %s`, p.prevDir, currentDir)
			eq, err := StoppableConfigDirsComparison(p.prevDir, currentDir, &p.doStop)
			if err == compareStoppedErr {
				glog.Info("Config dir comparison stopped")
				// We expect to receive stoppedCh soon, make sure we don't resume
				// (easily).
				p.firstDir = ""
				p.prevDir = ""
				continue
			} else if err != nil {
				glog.Errorf(`Errors while comparing config dirs:
previous: %s
 current: %s
   error: %s`, p.prevDir, currentDir, err)
   			// Since we don't know if the error is with the prevDir or currentDir,
   			// keep them both for later analysis.
				p.firstDir = ""
				p.prevDir = ""
				continue
			} else if !eq {
				glog.Infoln("Configurations are different")
				continue
			}
			glog.Infoln("Configurations are the same")
			if p.firstDir == p.prevDir {
				glog.Info("firstDir and prevDir are the same, no middle dir yet")
				p.prevDir = currentDir
				continue
			}
			glog.Infoln("Removing middle config dir", p.prevDir)
			doRemoveConfigDir(p.prevDir, &p.doStop)
			p.prevDir = currentDir
		}
	}
}

func (p *ConfigCuller) AddDir(newDir string) {
	p.recvCh <- newDir
}

func (p *ConfigCuller) RequestStop() chan bool {
	glog.Info("Requesting config culler to stop")
	atomic.StoreInt32(&p.doStop, 1)
	stoppedCh := make(chan bool)
	p.stopCh <- stoppedCh
	return stoppedCh
}

func (p *ConfigCuller) Stop() {
	ch := p.RequestStop()
	<-ch
	glog.Info("Config culler stopped")
}

func NewConfigCuller() *ConfigCuller {
	state := &ConfigCuller{
		stopCh: make(chan chan bool),
		recvCh: make(chan string),
	}
	go runCuller(state)
	return state
}
