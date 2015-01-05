package nblocations

// Support for aggregating location reports (for overly precise determination
// of report time), sorting by time, and writing to CSV file (gzipped).

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
	"os"
	"path/filepath"
	"time"
)

type ArchiveSplitPoint interface {
	NextArchiveSplitPoint(t time.Time) time.Time
}

type ArchiveOpener interface {
	OpenArchiveFor(t time.Time) (string, *util.CsvWriteCloser, error)
}

type ArchiveSplitterOpener interface {
	ArchiveSplitPoint
	ArchiveOpener
}

type dailyArchiveSplitterOpener string

func MakeDailyArchiveSplitterOpener(root_dir string) ArchiveSplitterOpener {
	return dailyArchiveSplitterOpener(root_dir)
}

func (root_dir dailyArchiveSplitterOpener) NextArchiveSplitPoint(t time.Time) time.Time {
	t2 := t.AddDate(0, 0, 1)
	return time.Date(t2.Year(), t2.Month(), t2.Day(), 0, 0, 0, 0, t2.Location())
}

func (root_dir dailyArchiveSplitterOpener) OpenArchiveFor(t time.Time) (
	string, *util.CsvWriteCloser, error) {
	dir := filepath.Join(string(root_dir), t.Format("2006"), t.Format("01"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, fmt.Errorf("Unable to create directory: %q\nError: %v", dir, err)
	}
	base := t.Format("2006-01-02")
	ext := ".csv.gz"
	f, path, err := util.OpenUniqueFile(dir, base, ext, 0755, 0444)
	if err != nil {
		return "", nil, err
	}
	csv := util.NewCsvWriteCloser(f, true)
	glog.Info("Created ", path)
	return path, csv, nil
}

// A new archive every 2 minutes
type debugArchiveSplitterOpener string

const twoMinutes = 2 * time.Minute

func MakeDebugArchiveSplitterOpener(root_dir string) ArchiveSplitterOpener {
	return debugArchiveSplitterOpener(root_dir)
}

func (root_dir debugArchiveSplitterOpener) NextArchiveSplitPoint(t time.Time) time.Time {
	t2 := t.Add(twoMinutes)
	t3 := t2.Truncate(twoMinutes)
	glog.Infof("NextArchiveSplitPoint\n  t: %s\n t2: %s\n t3: %s", t, t2, t3)
	return t3
}

func (root_dir debugArchiveSplitterOpener) OpenArchiveFor(t time.Time) (
	string, *util.CsvWriteCloser, error) {
	dir := filepath.Join(string(root_dir),
		t.Format("2006"), t.Format("01"), t.Format("02"), t.Format("15"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, fmt.Errorf("Unable to create directory: %s\nError: %v", dir, err)
	}
	base := t.Format("2006-01-02_1504")
	ext := ".csv.gz"
	f, path, err := util.OpenUniqueFile(dir, base, ext, 0755, 0444)
	if err != nil {
		return "", nil, err
	}
	csv := util.NewCsvWriteCloser(f, true)
	glog.Infof("Created %s", path)
	return path, csv, nil
}

type CSVArchiver interface {
	// Closes the current archive file (if any).
	Close() error
	// Flushes the current archive file (if any).
	Flush() error
	// Flushes the current archive file (if any) (but not gzip buffers).
	PartialFlush() error

	Write(location *nextbus.VehicleLocation) error
	WriteLocations(locations []*nextbus.VehicleLocation) error
}

type csvArchiver struct {
	opener   ArchiveOpener
	splitter ArchiveSplitPoint

	nextArchiveTime time.Time
	csv             *util.CsvWriteCloser
	csv_path        string
}

func MakeCSVArchiver(opener ArchiveOpener, splitter ArchiveSplitPoint) CSVArchiver {
	return &csvArchiver{
		opener:   opener,
		splitter: splitter,
	}
}

func (p *csvArchiver) Close() error {
	if p.csv != nil {
		err := p.csv.Close()
		p.csv = nil
		if p.csv_path != "" {
			glog.Infof("Closed %s", p.csv_path)
			p.csv_path = ""
		}
		return err
	}
	return nil
}

func (p *csvArchiver) Flush() error {
	if p.csv != nil {
		return p.csv.Flush()
	}
	return nil
}

func (p *csvArchiver) PartialFlush() error {
	if p.csv != nil {
		return p.csv.PartialFlush()
	}
	return nil
}

func (p *csvArchiver) shouldSplit(location *nextbus.VehicleLocation) bool {
	if p.csv == nil {
		return true
	}
	if p.nextArchiveTime.Before(location.Time) {
		return true
	}
	return false
}

func (p *csvArchiver) Write(location *nextbus.VehicleLocation) error {
	if p.shouldSplit(location) {
		p.Close()
		path, csv, err := p.opener.OpenArchiveFor(location.Time)
		if err != nil {
			return err
		}
		p.csv = csv
		p.csv_path = path
		p.nextArchiveTime = p.splitter.NextArchiveSplitPoint(location.Time)
		// Add a header identifying the fields.
		fieldNames := nextbus.VehicleCSVFieldNames()
		fieldNames[0] = "# " + fieldNames[0]
		p.csv.Write(fieldNames)
	}
	return p.csv.Write(location.ToCSVFields())
}

func (p *csvArchiver) WriteLocations(locations []*nextbus.VehicleLocation) error {
	var errors []error
	for _, location := range locations {
		if err := p.Write(location); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("WriteLocations failed:\n%s", util.JoinErrors(errors, "\n"))
	}
	return nil
}
