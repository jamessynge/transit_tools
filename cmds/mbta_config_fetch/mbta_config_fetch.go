package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/jamessynge/transit_tools/nextbus/configfetch"
	"github.com/jamessynge/transit_tools/util"
	"net/http"
	"time"
)

func main() {
	flag.Parse()
	glog.Info("Starting...")
	util.InitGOMAXPROCS()

	// Nextbus limit is 2MB/20seconds (100KB/second), and we want most of that
	// to be available for dynamic data, so cap this at a 20KB/second
	// (400KB/20seconds).
	var capacity uint32 = 400 * 1024
	duration := time.Duration(20) * time.Second
	rr, err := util.NewRateRegulator(capacity/4, capacity, duration)
	if err != nil {
		glog.Fatal(err)
	}
	client := &http.Client{}
	fetcher := util.NewRegulatedHttpFetcher(client, rr, true)
	err = configfetch.FetchAgencyConfig("mbta", `c:\temp\mbta_config_fetch_test`, fetcher)
	if err != nil {
		glog.Fatal(err)
	}
}
