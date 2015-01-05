package util

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

type HttpFetchResponse struct {
	StartTime, ResponseTime, CloseTime time.Time
	Response                           *http.Response
	ResponseErr                        error
	Body                               []byte
	BodyErr                            error
}

func DoHttpRequest(client *http.Client, request *http.Request) *HttpFetchResponse {
	glog.V(1).Infoln(request.Method, request.URL)

	hfr := &HttpFetchResponse{
		StartTime: time.Now(),
	}
	response, responseErr := client.Do(request)
	hfr.ResponseTime = time.Now()
	hfr.Response = response
	hfr.ResponseErr = responseErr

	if responseErr != nil && glog.V(1) {
		glog.Infof("Error executing http.Client.Do\nURL: %s\nError: %s",
			request.URL, responseErr)
	}

	if response != nil && response.Body != nil {
		body, bodyErr := ioutil.ReadAll(response.Body)
		response.Body.Close()
		hfr.CloseTime = time.Now()
		if bodyErr != nil {
			if len(body) > 10000 {
				body = body[0:4096]
			}
			glog.Infof(
				"Error reading response body.\nURL: %s\nError: %s\nBody: %q",
				request.URL, bodyErr, body)
			body = nil // Can't trust it.
		}
		hfr.Body = body
		hfr.BodyErr = bodyErr
	} else {
		hfr.CloseTime = hfr.ResponseTime
	}

	if glog.V(2) {
		reqSize, _ := EstimateRequestSize(request)
		respSize, _ := hfr.EstimateSize()
		glog.Infof("DoHttpRequest: reqSize=%d respSize=%d totalSize=%d\nURL: %s",
			reqSize, respSize, reqSize+respSize, request.URL)
	}

	if response != nil && response.StatusCode != http.StatusOK {
		glog.V(1).Infof("Unusual response status: %s\nURL: %s",
			response.Status, request.URL)
	}
	return hfr
}

func (p *HttpFetchResponse) EstimateSize() (size64 uint64, err error) {
	var w CountingBitBucketWriter
	if p.Response != nil {
		err = p.Response.Write(&w)
	}
	size64 = w.Size() + uint64(len(p.Body))
	return
}

var httpSummaryCleaner = strings.NewReplacer(
	"\n", "\\n", "\r", "\\r", "\\", "\\\\")
var zeroTime time.Time

func (p *HttpFetchResponse) WriteSummary(
	w io.Writer, ignoreFn func(key, value string) bool) {
	timeLayout := "2006-01-02T15:04:05.999Z07:00"
	if zeroTime != p.StartTime {
		fmt.Fprintf(w, "StartTime=%s\n", p.StartTime.Format(timeLayout))
	}
	if zeroTime != p.ResponseTime {
		fmt.Fprintf(w, "ResponseTime=%s\n", p.ResponseTime.Format(timeLayout))
	}
	if zeroTime != p.CloseTime {
		fmt.Fprintf(w, "CloseTime=%s\n", p.CloseTime.Format(timeLayout))
	}
	//	if p.WaitFor > 0 {
	//		fmt.Fprintf(w, "WaitFor=%s\n", p.WaitFor)
	//	}
	if p.ResponseErr != nil {
		fmt.Fprintf(w, "ResponseErr=%s\n", p.ResponseErr)
	}
	if p.BodyErr != nil {
		fmt.Fprintf(w, "BodyErr=%s\n", p.BodyErr)
	}
	if p.Response != nil {
		skipStandardComments := true
		if p.Response.StatusCode != http.StatusOK {
			skipStandardComments = false
			fmt.Fprintf(w, "Status=%s\n", p.Response.Status)
			fmt.Fprintf(w, "StatusCode=%s\n", p.Response.StatusCode)
		}
		first := true
		if ignoreFn == nil || skipStandardComments {
			ignoreFn = func(key, value string) bool { return false }
		}
		for _, kvs := range SortHeaderItems(p.Response.Header) {
			cleanedKey := httpSummaryCleaner.Replace(kvs.Key)
			for _, value := range kvs.Values {
				if !ignoreFn(kvs.Key, value) {
					if first {
						fmt.Fprintln(w)
						first = false
					}
					fmt.Fprintf(w, "%s: %s\n", cleanedKey,
						httpSummaryCleaner.Replace(value))
				}
			}
		}
	}
}

type HttpFetcher interface {
	// Error return if the fetcher is already closed, or if there was an error
	// from the underlying http.Client.Do.
	Do(request *http.Request) (*HttpFetchResponse, error)

	// Close the fetcher, so it will accept no more fetch requests.
	Close()
}

type simpleHttpFetchRequest struct {
	request  *http.Request
	response *HttpFetchResponse
	outCh    chan<- *simpleHttpFetchRequest
}

func simpleHttpFetcherFunc(
	client *http.Client, requests <-chan *simpleHttpFetchRequest,
	closeCh chan chan bool) {
	for {
		select {
		case ch, ok := <-closeCh:
			if ok && ch != nil {
				ch <- true
			}
			return
		case sfr, ok := <-requests:
			if !ok {
				if closeCh != nil {
					glog.Warning("requests channel closed unexpectedly!")
				}
				return
			}
			sfr.response = DoHttpRequest(client, sfr.request)
			sfr.outCh <- sfr
		}
	}
}

type regulatedHttpFetcherState struct {
	regulator RateRegulator
	inCh      chan<- *simpleHttpFetchRequest
	closeCh   chan chan bool
	doWait    bool
}

func (p *regulatedHttpFetcherState) Do(
	request *http.Request) (*HttpFetchResponse, error) {
	startTime := time.Now()

	if p.closeCh == nil {
		return nil, errors.New("fetcher is closed")
	}

	ch := make(chan *simpleHttpFetchRequest)
	sfr := &simpleHttpFetchRequest{
		outCh:   ch,
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
	//	sfr.response.WaitFor = waitFor
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
	<-doneCh
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
		inCh:      inCh,
		closeCh:   closeCh,
		doWait:    doWait,
	}
	go simpleHttpFetcherFunc(client, inCh, closeCh)
	return state
}
