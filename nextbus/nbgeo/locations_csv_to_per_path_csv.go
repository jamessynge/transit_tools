package nbgeo

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/golang/glog"

	//	"github.com/jamessynge/transit_tools/geom"
	"github.com/jamessynge/transit_tools/nextbus"
	"github.com/jamessynge/transit_tools/util"
)

// Support for sending records to an appropriate output file, each with
// its own output channel.
type CsvOutputChanMap struct {
	outDir      string
	delExisting bool
	perm        os.FileMode
	fileChans   map[int]chan []string
	closedWG    sync.WaitGroup
	mutex       sync.RWMutex
}

func NewCsvOutputChanMap(
	outDir string, delExisting bool, perm os.FileMode) *CsvOutputChanMap {
	return &CsvOutputChanMap{
		outDir:      outDir,
		delExisting: delExisting,
		perm:        perm,
		fileChans:   make(map[int]chan []string),
	}
}

func recordWriter(ch chan []string, wc *util.CsvWriteCloser,
	filePath string, closedWG *sync.WaitGroup) {
	defer closedWG.Done()
	defer wc.Close()

	count := 0
	nextAnnounce := 100
	announceStep := 100
	for {
		record, ok := <-ch
		if !ok {
			glog.Infof("After %d records, closing %s", count, filePath)
			return
		}
		err := wc.Write(record)
		if err != nil {
			glog.Warningf("Error writing to %s\n\t%v", filePath, err)
			return
		}
		count++
		if count >= nextAnnounce {
			glog.Infof("Wrote %d records to %s", count, filePath)
			if nextAnnounce == 10 * announceStep {
				announceStep = nextAnnounce
			}
			nextAnnounce += announceStep
			wc.Flush()
		}
	}
}

func (p *CsvOutputChanMap) Lookup(ndx int) (ch chan []string, err error) {
	if ndx <= 0 {
		ndx = 0
	}
	p.mutex.RLock()
	ch, ok := p.fileChans[ndx]
	p.mutex.RUnlock()
	if ok {
		return
	}
	p.mutex.Lock()
	ch, ok = p.fileChans[ndx]
	if !ok {
		//		log.Printf("Have no file open for path index %d", ndx)
		var filePath string
		if ndx <= 0 {
			filePath = filepath.Join(
				p.outDir, "path_unknown_locations.csv.gz")
		} else {
			filePath = filepath.Join(
				p.outDir, fmt.Sprintf("path_%04d_locations.csv.gz", ndx))
		}
		wc, err := util.OpenCsvWriteCloser(filePath, true, true, 0666)
		if err != nil {
			return nil, err
		}
		//		log.Printf("Opened for writing: %s", filePath)
		ch = make(chan []string, 10)
		p.closedWG.Add(1)
		go recordWriter(ch, wc, filePath, &p.closedWG)
		p.fileChans[ndx] = ch
	}
	p.mutex.Unlock()
	return ch, nil
}

func (p *CsvOutputChanMap) Write(ndx int, record []string) error {
	ch, err := p.Lookup(ndx)
	if err != nil {
		return err
	}
	ch <- record
	return nil
}

func (p *CsvOutputChanMap) CloseAll() {
	p.mutex.Lock()
	for _, ch := range p.fileChans {
		// TODO Need a way to track when all recordWriters have finished writing.
		close(ch)
	}
	p.closedWG.Wait()
	p.fileChans = make(map[int]chan []string)
	p.mutex.Unlock()
}

func locationsToPerPathFilesWorker(
	qtm *RouteToQuadTreeMap,
	inputChan <-chan []string,
	outputChans *CsvOutputChanMap,
	workerWG *sync.WaitGroup) {
	defer workerWG.Done()
	for {
		record, ok := <-inputChan
		if !ok {
			return
		}
		vl, err := nextbus.CSVFieldsToVehicleLocation(record)
		if err != nil {
			glog.Warningf("Error parsing field location record: %v\n\terr: %v",
				record, err)
			// TODO Could add an error output channel/file to make it easier to
			// debug later.
			continue
		}
		paths := qtm.VLToPaths(vl, 0.00067, 0.00042)
		if len(paths) == 0 {
			//			log.Printf("Did NOT match any paths: %v", vl)
			err = outputChans.Write(-1, record)
			if err != nil {
				glog.Warningf("Failed to write record, error: %v", err)
			}
		} else {
			for _, path := range paths {
				err = outputChans.Write(path.Index, record)
				if err != nil {
					glog.Warningf("Failed to write record, error: %v", err)
				}
			}
		}
	}
}

func LocationsFileToPerPathFiles(
	qtm *RouteToQuadTreeMap,
	filePath string,
	outputChans *CsvOutputChanMap,
	fileWG *sync.WaitGroup,
	numWorkers int) (numRecords int, err error) {
	defer fileWG.Done()
	maxProcs := runtime.GOMAXPROCS(-1)
	if numWorkers < 1 {
		numWorkers = maxProcs
	} else if numWorkers > maxProcs {
		numWorkers = maxProcs
	}
	recordChan := make(chan []string, 20*numWorkers)
	workerWG := &sync.WaitGroup{}
	workerWG.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go locationsToPerPathFilesWorker(qtm, recordChan, outputChans, workerWG)
	}
	numRecords, err = util.ReadCsvFileToChan(filePath, recordChan)
	close(recordChan)
	workerWG.Wait()
	return
}
