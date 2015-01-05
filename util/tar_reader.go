package util

import (
	"archive/tar"
	"io"

	"github.com/golang/glog"
)

// Function that will be called for each entry. body is only valid while in
// this function. If an error is returned, the caller will stop processing
// the archive and return that error (except for EOF, which will be replaced
// with nil).
type EntryFunc func(header *tar.Header, body io.Reader) error

// Read a Tape Archive, passes entries to a function on at a time.
func ProcessTarEntries(tr *tar.Reader, ef EntryFunc) error {
	entryNum := 0
	lastName := ""
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			if entryNum > 0 {
				glog.Warningf("Error reading header of entry %d: %s\nLast name: %s", entryNum, err, lastName)
			} else {
				glog.Warningf("Error reading header of entry %d: %s", entryNum, err)
			}
			return err
		}
		thisName := header.Name
		err = ef(header, tr)
		if err == nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		entryNum++
		lastName = thisName
	}
}

// Reads a Tape ARchive, passing entries to a function one at a time.
func ProcessTarFile(archivePath string, ef EntryFunc) error {
	rc, err := OpenReadFile(archivePath)
	if err != nil {
		return err
	}
	defer rc.Close()
	tr := tar.NewReader(rc)
	return ProcessTarEntries(tr, ef)
}

type TarHeaderAndData struct {
	tar.Header
	Body []byte
}

func MakeTarEntryToChannelFunc(ch chan *TarHeaderAndData) EntryFunc {
	return func(header *tar.Header, reader io.Reader) error {
		data := &TarHeaderAndData{
			Header: *header,
			Body:   make([]byte, header.Size),
		}
		data.Xattrs = make(map[string]string)
		for k, v := range header.Xattrs {
			data.Xattrs[k] = v
		}
		offset := int64(0)
		remaining := header.Size
		for remaining > 0 {
			n, err := reader.Read(data.Body[offset:])
			if n > 0 {
				offset += int64(n)
				remaining -= int64(n)
			}
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}
		}
		ch <- data
		return nil
	}
}
