package util

import (
	"compress/gzip"
	"encoding/csv"
	"os"

	"github.com/golang/glog"
)

type CsvWriteCloser struct {
	fwc *os.File
	gzw *gzip.Writer
	cw  *csv.Writer
}

func NewCsvWriteCloser(fwc *os.File, compress bool) *CsvWriteCloser {
	r := &CsvWriteCloser{fwc: fwc}
	if compress {
		r.gzw = gzip.NewWriter(fwc)
		r.cw = csv.NewWriter(r.gzw)
	} else {
		r.cw = csv.NewWriter(fwc)
	}
	return r
}

// OpenCsvWriteCloser opens a file for writing CSV records. If compress is true,
// the output is compressed using gzip. If delExisting is true and the file
// exists when this function is called, OpenCsvWriteCloser attempts to delete
// the file before opening the file; if this fails the function attempts
// to truncate the existing file as it opens it.
// perm specifies the os.FileMode to be used when creating the file.
func OpenCsvWriteCloser(
	filePath string, compress, delExisting bool, perm os.FileMode) (*CsvWriteCloser, error) {
	glog.V(1).Infof("OpenCsvWriteCloser(%q, compress=%v, delExisting=%v, %b)",
		filePath, compress, delExisting, perm)

	if delExisting && Exists(filePath) {
		if err := os.Remove(filePath); err == nil {
			glog.V(1).Infof("Deleted existing file %s", filePath)
		} else {
			glog.Warningf("Unable to delete existing file %s", filePath)
		}
	}
	//	flag := os.O_WRONLY
	//	if Exists(filePath) {
	//		if delExisting {
	//			flag |= os.O_TRUNC
	//		} else {
	//			flag |= os.O_APPEND
	//		}
	//	} else {
	//		flag |= os.O_CREATE
	//	}
	flag := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if delExisting {
		flag = flag | os.O_TRUNC
	}
	fwc, err := os.OpenFile(filePath, flag, perm)
	if err != nil {
		return nil, err
	}
	return NewCsvWriteCloser(fwc, compress), nil
}

// Writer writes a single CSV record to p along with any necessary quoting.
// A record is a slice of strings with each string being one field.
func (p *CsvWriteCloser) Write(record []string) error {
	return p.cw.Write(record)
}

// Flush buffered content (but not os.File.Sync nor syscall.Fsync).
func (p *CsvWriteCloser) Flush() error {
	return p.flush(true)
}

// Don't flush GZip so we don't screw with performance of compressor.
func (p *CsvWriteCloser) PartialFlush() error {
	return p.flush(false)
}

func (p *CsvWriteCloser) flush(fullFlush bool) error {
	p.cw.Flush()
	if fullFlush && p.gzw != nil {
		err := p.gzw.Flush()
		if err != nil {
			return err
		}
	}
	err := p.fwc.Sync()
	if err != nil {
		return err
	}
	return p.cw.Error()
}

// Flush the buffers, and close the underlying file.
func (p *CsvWriteCloser) Close() error {
	p.cw.Flush()
	if p.gzw != nil {
		err1 := p.gzw.Close()
		err2 := p.fwc.Close()
		if err2 != nil {
			return err2
		}
		if err1 != nil {
			return err1
		}
	} else {
		err := p.fwc.Close()
		if err != nil {
			return err
		}
	}
	return p.cw.Error()
}
