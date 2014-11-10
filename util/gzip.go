package util

import (
	"compress/gzip"
	"github.com/golang/glog"
	"io"
	"os"
	"strings"
)

type gzipReadCloser struct {
	src io.ReadCloser
	gr  *gzip.Reader
}

func (grc *gzipReadCloser) Read(p []byte) (n int, err error) {
	return grc.gr.Read(p)
}

func (grc *gzipReadCloser) Close() (err error) {
	defer func() {
		err2 := grc.src.Close()
		if err == nil {
			err = err2
		}
	}()
	err = grc.gr.Close()
	return
}

func OpenReadFile(filePath string) (rc io.ReadCloser, err error) {
	// Open the file for reading.
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	if !strings.HasSuffix(filePath, ".gz") {
		glog.V(1).Infof("Opened file for reading: %s", filePath)
		return f, nil
	}
	gr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return
	}
	rc = &gzipReadCloser{f, gr}
	glog.V(1).Infof("Opened compressed file for reading: %s", filePath)
	return
}
