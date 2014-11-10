package util

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io"
	"os"
	"runtime"
)

var (
	goMaxProcsFlag = flag.Int(
		"go-max-procs", runtime.NumCPU(),
		"Number of concurrent go routines to run concurrently")
)

func InitGOMAXPROCS() {
	cpus := runtime.NumCPU()
	fmt.Println("NumCPU:", cpus)
	max := runtime.GOMAXPROCS(*goMaxProcsFlag)
	fmt.Println("Original GOMAXPROCS:", max)
	max = runtime.GOMAXPROCS(-1)
	fmt.Println("Current GOMAXPROCS:", max)
}

func LogToStderrValue() (logtostderr, ok bool) {
	logtostderr_flag := flag.Lookup("logtostderr")
	if logtostderr_flag == nil {
		ok = false
		return
	}
	getter, ok := logtostderr_flag.Value.(flag.Getter)
	if !ok {
		return
	}
	logtostderr, ok = getter.Get().(bool)
	return
}

// glog doesn't look at the value of log_dir except when it is first logging
// a message of a higher severity than previously logged, or when it is
// rotating log files.  So, call this immediately after parsing the command
// line (i.e. before logging).
// Note that we don't actually set the default value of the flag because that
// won't be examined by glog when the flag is unset.
func SetDefaultLogDir(default_log_dir string) {
	/*if logtostderr, ok := LogToStderrValue(); ok && logtostderr {
		// No need to set it because logging to stderr.
		return
	}*/
	//	fmt.Printf("SetDefaultLogDir(%q)\n", default_log_dir)
	log_dir_flag := flag.Lookup("log_dir")
	if log_dir_flag == nil {
		// Flag doesn't exist!
		fmt.Println("Flag doesn't exist!")
		return
	}
	if len(log_dir_flag.Value.String()) > 0 {
		fmt.Println("Already set.")
		// Already set.
		return
	}
	if !IsDirectory(default_log_dir) {
		if err := os.MkdirAll(default_log_dir, 0750); err != nil {
			glog.Fatalf(`Unable to create log directory!
 Path: %q
Error: %s`, default_log_dir, err)
		}
	}
	fmt.Printf("Setting --log_dir to %v\n", default_log_dir)

	log_dir_flag.Value.Set(default_log_dir)
	glog.V(1).Infof("Set --log_dir to %q", default_log_dir)
}

// Generalization of ioutil.WriteFile to handle data that is split into
// fragments (i.e. is a slice of byte slices).
func WriteFile(fp string, fragments [][]byte, perm os.FileMode) (err error) {
	var f *os.File
	if f, err = os.OpenFile(fp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm); err != nil {
		return
	}
	for _, fragment := range fragments {
		if len(fragment) == 0 {
			continue
		}
		var n int
		n, err = f.Write(fragment)
		if err != nil {
			break
		}
		if n != len(fragment) {
			err = io.ErrShortWrite
			break
		}
	}
	err2 := f.Close()
	if err == nil {
		err = err2
	}
	return
}

type CountingBitBucketWriter uint64

func (p *CountingBitBucketWriter) Write(data []byte) (int, error) {
	size := len(data)
	*p = *p + CountingBitBucketWriter(size)
	return size, nil
}

func (p *CountingBitBucketWriter) Size() uint64 {
	return uint64(*p)
}
