package util

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"github.com/golang/glog"
	"os"
	"path/filepath"
	"strings"
)

type TarWriter struct {
	file *os.File
	buf  *bufio.Writer
	gzip *gzip.Writer
	tar  *tar.Writer
	path string
}

func NewTarWriter(file *os.File, compress bool) *TarWriter {
	p := &TarWriter{file: file}
	p.buf = bufio.NewWriterSize(p.file, 64*1024)
	if compress {
		p.gzip, _ = gzip.NewWriterLevel(p.buf, gzip.BestCompression)
		p.tar = tar.NewWriter(p.gzip)
	} else {
		p.tar = tar.NewWriter(p.buf)
	}
	return p
}

// TODO Add param to specify APPENDING to an existing file (may require
// validating the existing file first); or OVERWRITING existing files;
// or ADDING a numeric suffix to the basename (e.g. dir/base_123.ext)
// to make the path unique; if none specified, and file exists,
// return an error.
func MakeTarWriter(path string) (*TarWriter, error) {
	dir := filepath.Dir(path)
	if !Exists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			glog.Errorf("Unable to create directory: %s\nError: %v", dir, err)
			return nil, err
		}
	}
	if Exists(path) {
		glog.Infof("Replacing older archive: %s", path)
		if err := os.Remove(path); err != nil {
			glog.Errorf("Unable to delete older output: %s\nError: %v", path, err)
			return nil, err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE, os.FileMode(0644))
	if err != nil {
		glog.Errorf("Unable to open: %s\nError: %v", path, err)
		return nil, err
	}
	compress := strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".tgz")
	p := NewTarWriter(file, compress)
	p.path = path
	return p, nil
}

func (p *TarWriter) returnClosedError() error {
	if len(p.path) != 0 {
		return errors.New("Already closed " + p.path)
	} else {
		return errors.New("TarWriter already closed")
	}
}

func (p *TarWriter) WriteHeader(hdr *tar.Header) error {
	if p.tar == nil {
		return p.returnClosedError()
	}
	return p.tar.WriteHeader(hdr)
}

func (p *TarWriter) Write(b []byte) (n int, err error) {
	if p.tar == nil {
		return 0, p.returnClosedError()
	}
	return p.tar.Write(b)
}

func (p *TarWriter) WriteHeaderAndParts(
	hdr *tar.Header, parts [][]byte) (int64, error) {
	if p.tar == nil {
		return 0, p.returnClosedError()
	}
	if err := p.tar.WriteHeader(hdr); err != nil {
		return 0, err
	}
	var total int64 = 0
	for _, part := range parts {
		n, err := p.tar.Write(part)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (p *TarWriter) Flush() error {
	if p.tar != nil {
		if err := p.tar.Flush(); err != nil {
			return err
		}
	}
	if p.gzip != nil {
		if err := p.gzip.Flush(); err != nil {
			return err
		}
	}
	if p.buf != nil {
		if err := p.buf.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// Don't flush gzip in case that messes with compression.
func (p *TarWriter) PartialFlush() error {
	if p.tar != nil {
		if err := p.tar.Flush(); err != nil {
			return err
		}
	}
	if p.buf != nil {
		if err := p.buf.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (p *TarWriter) Close() error {
	if p.tar != nil {
		if err := p.tar.Close(); err != nil {
			return err
		}
		p.tar = nil
	}
	if p.gzip != nil {
		if err := p.gzip.Close(); err != nil {
			return err
		}
		p.gzip = nil
	}
	if p.buf != nil {
		if err := p.buf.Flush(); err != nil {
			return err
		}
		p.buf = nil
	}
	if p.file != nil {
		if err := p.file.Close(); err != nil {
			return err
		}
		p.file = nil
	}
	return nil
}
