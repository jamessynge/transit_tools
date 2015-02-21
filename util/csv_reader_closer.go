package util

import (
	"encoding/csv"
	"io"

	"github.com/golang/glog"
)

// Interface exposing csv.Reader operations Read (next record) and ReadAll (all
// remaining records from the underlying source).
// TODO(jamessynge): Move to a csv specific package (e.g. csvio), and rename
// to Reader.
type CsvReader interface {
	Read() (record []string, err error)
	ReadAll() (records [][]string, err error)
}

// A CsvReader implementation, adding a Close method. Embeds a csv.Reader,
// so the exported fields (Comma, Comment, etc.) of csv.Reader are exported.
// TODO(jamessynge):  Move to a csv specific package (e.g. csvio), and rename
// to ReadCloser (not ReaderCloser).
type CsvReaderCloser struct {
	src io.Closer
	*csv.Reader
}

// Close closes the underlying io.ReadCloser.
func (r *CsvReaderCloser) Close() (err error) {
	tmp := r.src
	r.Reader = nil
	r.src = nil
	if tmp != nil {
		err = tmp.Close()
	}
	return
}

// NewCsvReaderCloser returns a new CsvReaderCloser that reads from rc.
func NewCsvReaderCloser(rc io.ReadCloser) *CsvReaderCloser {
	return &CsvReaderCloser{
		src: rc,
		Reader: csv.NewReader(rc),
	}
}

// OpenReadCsvFile opens for reading a file (possibly compressed) of CSV records
// and returns a CsvReaderCloser.
func OpenReadCsvFile(filePath string) (crc *CsvReaderCloser, err error) {
	// Open the file for reading.
	rc, err := OpenReadFile(filePath)
	if err != nil {
		return
	}
	return NewCsvReaderCloser(rc), nil
}

func OpenReadCsvFileAndPump(filePath string) (crc *CsvReaderCloser, err error) {
	// Open the file for reading.
	rc, err := OpenReadFileAndPump(filePath)
	if err != nil {
		return
	}
	return NewCsvReaderCloser(rc), nil
}

// Process 1 record (or the error encountered when reading a record,
// including eof).
type RecordProcessorFn func(source string, record []string,
	recordNum int, err error) error

// If fn returns non-nil, ReadCsvToFn stops reading and returns that
// error (except for io.EOF, which is converted to nil before returning).
func ReadCsvToFn(r CsvReader, source string, fn RecordProcessorFn) (
	numRecords int, err error) {
	var record []string
	for {
		record, err = r.Read()
		if err == io.EOF {
			err = nil
			return
		}
		if err != nil {
			glog.Warningf("Error reading record %d from %s\nError: %s",
				numRecords+1, source, err)
		}
		err = fn(source, record, numRecords, err)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		numRecords++
	}
}

func ReadCsvFileToFn(filePath string, fn RecordProcessorFn) (
	numRecords int, err error) {
	crc, err := OpenReadCsvFile(filePath)
	if err != nil {
		glog.Warningf("Unable to open %s\nError: %s", filePath, err)
		return
	}
	defer func() {
		err2 := crc.Close()
		if err == nil {
			err = err2
			if err != nil {
				glog.Warningf("Error closing %s\nError: %s", filePath, err)
			}
		}
	}()
	crc.Comment = '#'
	return ReadCsvToFn(crc, filePath, fn)
}

func ReadCsvFileToChan(filePath string, ch chan<- []string) (
	numRecords int, err error) {
	return ReadCsvFileToFn(filePath,
		func(filePath string, record []string,
			recordNum int, err error) error {
			if err != nil {
				return err
			}
			ch <- record
			return nil
		})
}
