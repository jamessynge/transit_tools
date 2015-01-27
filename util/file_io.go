package util

import (
	"compress/gzip"
	"io"
	"os"
	"strings"
	"github.com/golang/glog"
)

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

func OpenReadFileAndPump(filePath string) (rc io.ReadCloser, err error) {
	// Open the file for reading.
	rc, err = os.Open(filePath)
	if err != nil {
		return
	}
	blockSize, blockCount := 4096, 16
	if strings.HasSuffix(filePath, ".gz") {
		glog.V(1).Infof("Opened compressed file for reading: %s", filePath)
		// Use half the blocks for the file pump, half for the decompressed pump
		// (not sure what the "right" ratio is, if such a thing exists).
		blockCount = 8
		rc = NewReadCloserPump(rc, blockSize, blockCount)
		gr, err2 := gzip.NewReader(rc)
		if err2 != nil {
			rc.Close()
			return nil, err2
		}
		rc = &gzipReadCloser{rc, gr}
	
	} else {
		glog.V(1).Infof("Opened file for reading: %s", filePath)
	}
	rc = NewReadCloserPump(rc, blockSize, blockCount)
	return rc, nil
}
