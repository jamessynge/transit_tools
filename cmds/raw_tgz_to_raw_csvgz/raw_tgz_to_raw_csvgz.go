package main

// Context:
//		Sometimes there is a drift in server time, sometimes > 60s, and
//	  this may be effecting the processed data written into the CSV files.
// Goal:
//		Read all the raw reports (XML, sometimes in compressed archives),
//		and emit raw CSVs, where instead of emitting the estimated time
//		at which the report was actually collected, we emit multiple columns
//		of time info (server, client, lasttime, report-age), and can later sort
//		by vehicle id and then server or client time to gather together the
//		entries that we should aggregate.
//
// Maybe first analyze the relationship of server, client and lasttime,
// to see if we can see the effect of server time drift on lasttime.
//
// Status:
//		Only a skeleton of the goal: just finds the compressed tar files.

import (
	"flag"
	//	"io"
	//	"io/ioutil"
	//	"math"
	"os"
	"path/filepath"
	//	"runtime"
	//	"sort"
	"strings"
	//	"time"
	"sync"

	"github.com/golang/glog"

	//	"github.com/jamessynge/transit_tools/nextbus"
	//	"github.com/jamessynge/transit_tools/nextbus/nblocations"
	"github.com/jamessynge/transit_tools/util"
)

var pRoots = flag.String(
	"roots",
	`C:\temp\fetch_vehicles*,C:\temp\nextbus_fetcher.*,C:\temp\raw-mbta-locations`,
	"Comma-separated list of file system locations to search.")

type PathAndInfo struct {
	Path string
	Info os.FileInfo
}

func emitRawFiles(root string, ch chan PathAndInfo) error {
	glog.V(1).Infoln("Searching under: ", root)
	errs := util.NewErrors()
	walkFn := func(fp string, info os.FileInfo, err error) error {
		glog.V(1).Infoln("walkFn", fp)
		if err != nil {
			errs.AddError(err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !(strings.HasSuffix(fp, ".tar.gz") || strings.HasSuffix(fp, ".tgz")) {
			return nil
		}
		fpl := strings.ToLower(fp)
		if !(strings.Contains(fpl, "raw") ||
			strings.Contains(fpl, "location") ||
			strings.Contains(fpl, "fetch")) {
			glog.Warning("Discarding non-(raw || location || fetch) file", fp)
			return nil
		}
		glog.V(1).Infoln("Keeping", fp)
		ch <- PathAndInfo{fp, info}
		return nil
	}
	err := filepath.Walk(root, walkFn)
	errs.AddError(err)
	return errs.ToError()
}

func main() {
	flag.Parse()
	util.InitGOMAXPROCS()

	roots, err := util.ExpandPathGlobs(*pRoots, ",")
	if err != nil {
		glog.Errorln("Error expanding --roots flag:", err)
		if len(roots) == 0 {
			os.Exit(1)
		}
	}

	piCh := make(chan PathAndInfo, 10)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, root := range roots {
			glog.Infoln("emitRawFiles", root)
			emitRawFiles(root, piCh)
		}
		close(piCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for pi := range piCh {
			glog.V(1).Infoln("Found", pi.Path)
		}
	}()

	wg.Wait()
}
