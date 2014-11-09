package util

import (
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"time"
)

type HiLoHttpFetcher interface {
	// Error return if the fetcher is already closed, or if there was an error
	// from the underlying http.Client.Do.
	Do(hiPriority bool, request *http.Request) (*HttpFetchResponse, error)

	// Close the fetcher, so it will accept no more fetch requests.
	Close()
}

type hiLoHttpFetcherState struct {
	client    *http.Client
	regulator RateRegulator
	hiCh      chan *simpleHttpFetchRequest
	loCh      chan *simpleHttpFetchRequest
	closeCh   chan chan bool
	closed    bool
}

func NewHiLoHttpFetcher(client *http.Client, regulator RateRegulator) HiLoHttpFetcher {
	state := &hiLoHttpFetcherState{
		client:    client,
		regulator: regulator,
		hiCh:      make(chan *simpleHttpFetchRequest),
		loCh:      make(chan *simpleHttpFetchRequest),
		closeCh:   make(chan chan bool),
	}
	go state.runHiLoHttpFetcher()
	return state
}

func (p *hiLoHttpFetcherState) Do(
	hiPriority bool, request *http.Request) (*HttpFetchResponse, error) {
	if p.closed {
		return nil, fmt.Errorf("Already closed")
	}
	ch := make(chan *simpleHttpFetchRequest)
	s := &simpleHttpFetchRequest{
		request: request,
		outCh:   ch,
	}
	if hiPriority {
		p.hiCh <- s
	} else {
		p.loCh <- s
	}
	s = <-ch
	err := s.response.ResponseErr
	if err == nil {
		err = s.response.BodyErr
	}
	return s.response, err
}

func (p *hiLoHttpFetcherState) Close() {
	if !p.closed {
		p.closed = true
		ch := make(chan bool)
		p.closeCh <- ch
		<-ch
	}
}

func (p *hiLoHttpFetcherState) runHiLoHttpFetcher() {
	// Use a rate regulated transport to add waits into the Write and Read
	// operations of the request/response round trip.
	var loClient http.Client = *p.client
	loClient.Transport = NewRateRegulatedTransport(p.regulator)
	var loCh chan *simpleHttpFetchRequest
	var pendingLoRequest *simpleHttpFetchRequest
	pendingLoRespCh := make(chan *HttpFetchResponse)

	// Use a rate regulated transport that just counts the amount of waiting
	// that should have happened, then delay the receiving of low priority
	// by that amount.
	noWaitRegulator := NewNoWaitRateRegulator(p.regulator)
	var hiClient http.Client = *p.client
	hiClient.Transport = NewRateRegulatedTransport(noWaitRegulator)

	// When the high priority requests run too fast, we delay starting low
	// priority requests using this timer and channel.  Initially we start
	// with loCh == nil, and when this short timer fires, we enable low
	// priority requests.
	permitLoTimer := time.NewTimer(1 * time.Millisecond)
	permitLoCh := permitLoTimer.C
	var delayAfterLoResp time.Duration

	// When we've been requested to close, we'll do so once there is no
	// low priority request in progress.
	var closeCh chan bool

	for {
		select {
		case hiReq, ok := <-p.hiCh:
			// High priority request
			if !ok {
				// Channel has been closed.
				p.hiCh = nil
				continue
			}

			hfr := DoHttpRequest(&hiClient, hiReq.request)
			hiReq.response = hfr
			hiReq.outCh <- hiReq

			waitFor := noWaitRegulator.DrainAccumulator()
			if waitFor <= 0 {
				continue
			}
			if pendingLoRequest == nil && loCh != nil {
				// Don't accept low priority request until waitFor has elapsed.
				loCh = nil
				permitLoTimer.Reset(waitFor)
				glog.Info("permitLoTimer: ", waitFor)
			} else {
				// There is a low priority request in progress.  Don't start another
				// until we've added an additional wait of waitFor.
				delayAfterLoResp += waitFor
				glog.Infoln("waitFor:", waitFor,
										"   delayAfterLoResp now: ", delayAfterLoResp)
			}

		case loReq, ok := <-loCh:
			// Low priority request.
			if !ok {
				loCh = nil
				p.loCh = nil
				continue
			}
			// Start a go routine to make the request in parallel with any high
			// priority requests that are received before it is done. The func
			// returns the response via pendingLoRespCh (next case).
			go func() {
				hfr := DoHttpRequest(&loClient, loReq.request)
				pendingLoRespCh <- hfr
			}()

			// Don't accept another low priority request until we're done with this
			// one.
			pendingLoRequest = loReq
			loCh = nil

		case hfr := <-pendingLoRespCh:
			// Completed a low priority request.
			pendingLoRequest.response = hfr
			pendingLoRequest.outCh <- pendingLoRequest
			pendingLoRequest = nil

			// Can we resume receiving low priority requests?
			if delayAfterLoResp > 0 {
				// Not yet.  Wait a bit.
				permitLoTimer.Reset(delayAfterLoResp)
				delayAfterLoResp = 0
				continue
			}
			// Yes.
			loCh = p.loCh

		case <-permitLoCh:
			// We are now permitted to process low priority requests.
			loCh = p.loCh
			delayAfterLoResp = 0

		case closeCh = <-p.closeCh:
			continue
		}

		if closeCh != nil && pendingLoRequest == nil {
			closeCh <- true
			permitLoTimer.Stop()
			return
		}
	}
}
