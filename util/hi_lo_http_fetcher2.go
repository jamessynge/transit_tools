package util

import (
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"time"
)

type hiLoHttpFetcher2State struct {
	hiFetcher HttpRegulatedFetcher
	loFetcher HttpRegulatedFetcher
	hiCh      chan *simpleHttpFetchRequest
	loCh      chan *simpleHttpFetchRequest
	closeCh   chan chan bool
	closed    bool
}

func NewHiLoHttpFetcher2(
	hiFetcher, loFetcher HttpRegulatedFetcher) HiLoHttpFetcher {
	state := &hiLoHttpFetcher2State{
		hiFetcher: hiFetcher,
		loFetcher: loFetcher,
		hiCh:      make(chan *simpleHttpFetchRequest),
		loCh:      make(chan *simpleHttpFetchRequest),
		closeCh:   make(chan chan bool),
	}
	go state.runHiLoHttpFetcher2()
	return state
}

func (p *hiLoHttpFetcher2State) Do(
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

func (p *hiLoHttpFetcher2State) Close() {
	if !p.closed {
		p.closed = true
		ch := make(chan bool)
		p.closeCh <- ch
		<-ch
	}
}

func (p *hiLoHttpFetcher2State) runHiLoHttpFetcher2() {
	var loCh chan *simpleHttpFetchRequest
	var pendingLoRequest *simpleHttpFetchRequest
	pendingLoRespCh := make(chan *HttpRegulatedFetchResponse)

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

			hrfr := p.hiFetcher.HttpRegulatedFetch(hiReq.request)
			hiReq.response = &(hrfr.HttpFetchResponse)
			hiReq.outCh <- hiReq

			if hrfr.WaitFor <= 0 {
				continue
			}
			if pendingLoRequest == nil && loCh != nil {
				// Don't accept low priority request until waitFor has elapsed.
				loCh = nil
				permitLoTimer.Reset(hrfr.WaitFor)
				glog.V(1).Info("permitLoTimer: ", hrfr.WaitFor)
			} else {
				// There is a low priority request in progress.  Don't start another
				// until we've added an additional wait of waitFor.
				delayAfterLoResp += hrfr.WaitFor
				glog.V(1).Infoln("WaitFor:", hrfr.WaitFor,
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
				hrfr := p.loFetcher.HttpRegulatedFetch(loReq.request)
				pendingLoRespCh <- hrfr
			}()
			pendingLoRequest = loReq

			// Don't accept another low priority request until we're done with this
			// one.
			loCh = nil

		case hrfr := <-pendingLoRespCh:
			// Completed a low priority request.
			pendingLoRequest.response = &(hrfr.HttpFetchResponse)
			pendingLoRequest.outCh <- pendingLoRequest
			pendingLoRequest = nil

			// Can we resume receiving low priority requests?
			delayAfterLoResp += hrfr.WaitFor
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
