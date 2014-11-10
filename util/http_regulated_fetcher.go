package util

import (
//	"github.com/golang/glog"
	"net/http"
	"time"
)

type HttpRegulatedFetchResponse struct {
	HttpFetchResponse
	WaitFor time.Duration
}

type HttpRegulatedFetcher interface {
	// Executes DoHttpRequest(*http.Client, *http.Request), using an http.Client
	// that the implementation provides, and returns the HttpFetchResponse, along
	// with an amount of time to wait as a result of performing this request.
	HttpRegulatedFetch(request *http.Request) *HttpRegulatedFetchResponse
}

type simpleHttpRegulatedFetcher struct {
	client *http.Client
	regulator RateRegulator
	doWait	 bool
}

func (p *simpleHttpRegulatedFetcher) HttpRegulatedFetch(
		request *http.Request) *HttpRegulatedFetchResponse {
	hfr := DoHttpRequest(p.client, request)
	duration := hfr.CloseTime.Sub(hfr.StartTime)
	resp := &HttpRegulatedFetchResponse{HttpFetchResponse: *hfr}
	// Assuming here that the response body is the part measured by a server that
	// wants us to limit the load on it (appears to be the case for NextBus).
	bodySize := len(hfr.Body)
	waitFor := p.regulator.Used(uint(bodySize), duration)
	if p.doWait {
		time.Sleep(waitFor)
	} else {
		resp.WaitFor = waitFor
	}
	return resp
}

func NewHttpRegulatedFetcher(
	client *http.Client, regulator RateRegulator, doWait bool) HttpRegulatedFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	state := &simpleHttpRegulatedFetcher{
		client: client,
		regulator: regulator,
		doWait:    doWait,
	}
	return state
}
