package util

/*
import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)




type regulatedHttpFetcherState struct {
	regulator RateRegulator
	inCh chan<- *simpleHttpFetchRequest
	closeCh chan chan bool
	doWait bool
}

func (p *regulatedHttpFetcherState) Do(
		request *http.Request) (*HttpFetchResponse, error) {
	startTime := time.Now()

	if p.closeCh == nil {
		return nil, errors.New("fetcher is closed")
	}

	ch := make(chan *simpleHttpFetchRequest)
	sfr := &simpleHttpFetchRequest{
		outCh: ch,
		request: request,
	}
	p.inCh <- sfr
	sfr = <-ch

	// Figure out how long to wait by measuring size of request and response,
	// and asking regulator.
	reqSize, err := EstimateRequestSize(request)
	respSize, err := sfr.response.EstimateSize()
	var size uint64 = reqSize + respSize

	// Cap the amount used at the max value for a uint.
	maxUsed := ^uint(0)
	if size > uint64(maxUsed) {
		size = uint64(maxUsed)
	}
	used := uint(size)

	period := time.Since(startTime)
	waitFor := p.regulator.Used(used, period)
	if p.doWait {
		glog.Infof("Waiting %s after fetching %s\nRequest size: %d\nResponse size: %d",
							 waitFor, request.URL, reqSize, respSize)
		time.Sleep(waitFor)
		waitFor = 0
	} else {
		glog.Infof("No need to wait after fetching %s\nRequest size: %d\nResponse size: %d",
							 request.URL, reqSize, respSize)
	}
	sfr.response.WaitFor = waitFor
	err = sfr.response.ResponseErr
	if err == nil {
		err = sfr.response.BodyErr
	}
	return sfr.response, err
}

func (p *regulatedHttpFetcherState) Close() {
	// Sucky design here.  Want a way to tell the goroutine to stop listening,
	// and ideally also to warn clients to stop sending requests here, but not
	// sure (yet) what the clean, ideomatic golang way to do that is. In the case
	// that we have multiple clients of an HttpFetcher, probably just need to
	// ensure they are shutdown first, but would like a way to communicate "back"
	// to such clients if they haven't been shutdown.

	closeCh := p.closeCh
	p.closeCh = nil
	if closeCh == nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			glog.Errorf("regulatedHttpFetcherState.Close() recovered from %v", p)
		}
	}()
	doneCh := make(chan bool)
	closeCh <- doneCh
	close(closeCh)
	<- doneCh
}

func NewRegulatedHttpFetcher(
		client *http.Client, regulator RateRegulator, doWait bool) HttpFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	inCh := make(chan *simpleHttpFetchRequest, 5)
	closeCh := make(chan chan bool)
	state := &regulatedHttpFetcherState{
		regulator: regulator,
		inCh: inCh,
		closeCh: closeCh,
		doWait: doWait,
	}
	go simpleHttpFetcherFunc(client, inCh, closeCh)
	return state
}

type hiPriorityHttpFetcherState struct {
	inCh chan<- *simpleHttpFetchRequest
	closeCh chan chan bool




	baseFetcher HttpFetcher




	regulator RateRegulator
	inCh chan<- *simpleHttpFetchRequest
	closeCh chan chan bool
	doWait bool
}

type loPriorityHttpFetcherState struct {
	baseFetcher HttpFetcher


	regulator RateRegulator
	inCh chan<- *simpleHttpFetchRequest
	closeCh chan chan bool
	doWait bool
}



// Returns two fetchers, one that waits, the other doesn't (for low and high
// priority callers); the waiting is pushed from the high to the low priority
// channel. It is assumed that there is only one user of each fetcher (not from
// a concurrency perspective, but from a moving of the waiting perspective).
func NewSemiRegulatedHttpFetchers(
		client *http.Client, regulator RateRegulator) HttpFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	inCh := make(chan *simpleHttpFetchRequest, 5)
	closeCh := make(chan chan bool)
	state := &regulatedHttpFetcherState{
		regulator: regulator,
		inCh: inCh,
		closeCh: closeCh,
		doWait: doWait,
	}
	go simpleHttpFetcherFunc(client, inCh, closeCh)
	return state
}
*/
