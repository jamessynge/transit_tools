package util

import (
	"compress/gzip"
	"io"
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
