package util

import (
	"encoding/csv"
	"io"
)

type CsvReaderCloser struct {
	src io.Closer
	cr  *csv.Reader
}

func (r *CsvReaderCloser) Read() (record []string, err error) {
	return r.cr.Read()
}
func (r *CsvReaderCloser) ReadAll() (records [][]string, err error) {
	return r.cr.ReadAll()
}
func (r *CsvReaderCloser) Close() (err error) {
	tmp := r.src
	r.cr = nil
	r.src = nil
	if tmp != nil {
		err = tmp.Close()
	}
	return
}
func NewCsvReaderCloser(rc io.ReadCloser) *CsvReaderCloser {
	return &CsvReaderCloser{rc, csv.NewReader(rc)}
}

func OpenReadCsvFile(filePath string) (crc *CsvReaderCloser, err error) {
	// Open the file for reading.
	rc, err := OpenReadFile(filePath)
	if err != nil {
		return
	}
	return NewCsvReaderCloser(rc), nil
}

func ReadCsvFileToChan(filePath string, ch chan<- []string) (
	numRecords int, err error) {
	crc, err := OpenReadCsvFile(filePath)
	if err != nil {
		return
	}
	defer func() {
		err2 := crc.Close()
		if err == nil {
			err = err2
		}
	}()

	var record []string
	for {
		record, err = crc.Read()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		ch <- record
		numRecords++
		//		if numRecords >= 1000 {
		//			log.Printf("ReadCsvFileToChan stopping early for debugging")
		//			return
		//		}
	}
}
