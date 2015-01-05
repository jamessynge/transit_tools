package util

import (
	"archive/tar"
	"fmt"
	"github.com/golang/glog"
	"path/filepath"
	"time"
)

// Creates tar files whose file path and entry names are based on time.Time
// values. The assumption is that entries are added in time order, with the
// time usually being roughly the current time.
type DatedTarArchiver struct {
	// File system directory under which all tar files are created.
	RootDir string

	// Layout for time.Time.Format which produces the fragment of the destination
	// tar file's path under RootDir.  For example:
	//      "2006/01/2006-01-02"
	// will be used to produce a full path like:
	//      "<RootDir>/2014/11/2014-11-04_003.tar.gz"
	// assuming Uncompressed is false, and the time of the entry being
	// added was on November 4, 2014, and that 3 previous files had already been
	// created for that date.
	// Defaults to "2006/01/2006-01-02" if unspecified, which will produce a new
	// tar when an entry is added with a different date than the previous entry.
	PathFragmentLayout string

	// Don't compress the tar file with gzip? Default is of course false, so
	// by default tar files will be compressed.
	Uncompressed bool

	// If 0, defaults to 0444 (read-only, world accessible).
	DefaultEntryMode int64

	// Full path of the current tar file.
	currentPath string
	// Fragment of current tar file path (see PathFragmentLayout).
	currentFragment string
	// TarWriter that provides write access to current tar file.
	currentTar *TarWriter
}

func (p *DatedTarArchiver) GetPathFragment(timestamp time.Time) string {
	if p.PathFragmentLayout == "" {
		p.PathFragmentLayout = "2006/01/2006-01-02"
	}
	return timestamp.Format(p.PathFragmentLayout)
}

func (p *DatedTarArchiver) GetTarWriter(
	timestamp time.Time) (*TarWriter, error) {
	fragment := p.GetPathFragment(timestamp)
	errs := NewErrors()
	if p.currentTar != nil {
		if p.currentFragment == fragment {
			return p.currentTar, nil
		}
		errs.AddError(p.Close())
	}

	glog.V(1).Infof("DatedTarArchiver:\nroot=%s\nfragment=%s",
		p.RootDir, fragment)

	dirFrag, baseFrag := filepath.Split(fragment)
	dir := filepath.Join(p.RootDir, dirFrag)

	file, path, err := OpenUniqueFile(
		dir, baseFrag, ".tar.gz", 0755, 0644)
	if err != nil {
		errs.AddError(err)
		return nil, errs.ToError()
	}

	glog.Info("Created ", path)

	p.currentPath = path
	p.currentFragment = fragment
	p.currentTar = NewTarWriter(file, !p.Uncompressed)
	return p.currentTar, nil
}

func (p *DatedTarArchiver) Flush() error {
	if p.currentTar != nil {
		return p.currentTar.Flush()
	}
	return nil
}

// Don't flush gzip in case that messes with compression.
func (p *DatedTarArchiver) PartialFlush() error {
	if p.currentTar != nil {
		return p.currentTar.PartialFlush()
	}
	return nil
}

func (p *DatedTarArchiver) Close() error {
	if p == nil || p.currentTar == nil {
		return nil
	}
	err := p.currentTar.Close()
	p.currentTar = nil
	glog.Infof("Closed %s", p.currentPath)
	p.currentPath = ""
	p.currentFragment = ""
	return err
}

func (p *DatedTarArchiver) AddHeaderAndParts(
	timestamp time.Time, hdr *tar.Header, parts [][]byte) error {
	errs := NewErrors()
	tw, err := p.GetTarWriter(timestamp)
	errs.AddError(err)
	if tw != nil {
		n, err2 := tw.WriteHeaderAndParts(hdr, parts)
		if err2 == nil && n != hdr.Size {
			err2 = fmt.Errorf(
				"AddHeaderAndParts: wrote %d bytes, not hdr.Size (%d) as expected",
				n, hdr.Size)
		}
		errs.AddError(err2)
	}
	err = errs.ToError()
	if err != nil {
		glog.Errorf("AddHeaderAndParts: %s", err)
	} else {
		glog.V(2).Infof("Wrote %s (%d bytes) to tar", hdr.Name, hdr.Size)
	}
	return err
}

func (p *DatedTarArchiver) AddFileParts(
	timestamp time.Time, filename string,
	parts [][]byte) error {
	var size int64 = 0
	for _, part := range parts {
		size += int64(len(part))
	}
	if p.DefaultEntryMode == 0 {
		p.DefaultEntryMode = 0444
	}
	hdr := &tar.Header{
		Name:    filename,
		Size:    size,
		ModTime: timestamp,
		Mode:    p.DefaultEntryMode,
	}
	return p.AddHeaderAndParts(timestamp, hdr, parts)
}
