package main 

import (
	"flag"
	"time"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/util"
)

func main() {
	flag.Parse()
	glog.Info("Starting...")
	util.InitGOMAXPROCS()
	time.Sleep(time.Second)

	testPartitionSize()
}
