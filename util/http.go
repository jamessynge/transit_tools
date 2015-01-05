package util

import (
	"errors"
	"flag"
	"github.com/golang/glog"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

// TODO Move failure injection out of here, and into consumer of Fetcher.
// DEBUG flags:
var inject_503_frac_flag = flag.Float64(
	"inject_503_frac", 0.0,
	"DEBUG: fraction of fetch responses to replace with a "+
		"503 Service Temporarily Unavailable.")
var inject_partial_read_frac_flag = flag.Float64(
	"inject_partial_read_frac", 0.0,
	"DEBUG: fraction of fetch responses to replace with a partial read of the "+
		"response body.")
var inject_timeout_frac_flag = flag.Float64(
	"inject_timeout_frac", 0.0,
	"DEBUG: fraction of fetch responses to replace with a timeout of the "+
		"request (e.g. no connection established, or no response received).")

func maybeInjectFetchFailure(response *http.Response, body []byte) (
	*http.Response, error, []byte, error) {
	total_debug_frac := (*inject_503_frac_flag + *inject_partial_read_frac_flag +
		*inject_timeout_frac_flag)

	if total_debug_frac <= 0 {
		return response, nil, body, nil
	}
	frac := rand.Float64()
	glog.V(1).Infof("total_debug_frac = %f, frac = %f", total_debug_frac, frac)
	if total_debug_frac <= frac {
		return response, nil, body, nil
	}
	if frac < *inject_503_frac_flag {
		glog.Warning("Injecting a 503 Service Temporarily Unavailable")
		response.StatusCode = 503
		response.Status = "503 Service Temporarily Unavailable"
		response.Header.Del("Content-Type")
		response.Header.Del("Content-Length")
		body = []byte(`<html>
<!-- This file is displayed when there is an internal server error (error 500)
     which happens when apache is running but Tomcat is not.  This is important
     because Tomcat takes several seconds to start being able to handle
     messages.  The idea is that this file displays something not
     so scary (instead of Internal Server Error) and then automatically
     does a refresh in 5 seconds to try to load in the page again.  This
     way the user doesn't have to do anything.  Don't want to do it too
     often because that would prevent the user from typing in another url.
 -->

<head>
<title>NextBus - Please wait...</title>
<meta http-equiv="Content-Type" content="text/html; charset=iso-8859-1">
<meta http-equiv="refresh" content="5">
</head>
<font face="Arial, Helvetica, sans-serif" size="2">Please wait...
</font>
</body>
</html>
`)
		return response, nil, body, nil
	}
	frac -= *inject_503_frac_flag

	if frac < *inject_partial_read_frac_flag {
		glog.Warning("Injecting a partial read")
		body = body[0 : len(body)/2]
		err := errors.New("Partial read of body (injected error for debugging)")
		return response, nil, body, err
	}
	frac -= *inject_partial_read_frac_flag

	if frac < *inject_timeout_frac_flag {
		glog.Warning("Injecting a response timeout")
		err := errors.New("Response timeout (injected error for debugging)")
		return nil, err, nil, nil
	}
	frac -= *inject_timeout_frac_flag

	// ... other errors ...
	// REMEMBER TO ADD THE FRAC TO total_debug_frac.

	return response, nil, body, nil
}

type FetcherRequest interface {
	// If the response from Request is nil, then no request is made, and Result
	// is not called.
	Request() *http.Request
	// Called after the fetcher has made the request.
	Result(response *http.Response, responseErr error, body []byte, bodyErr error)
}

func FetcherFunc(hiPriorityFetchRequestChan <-chan FetcherRequest,
	loPriorityFetchRequestChan <-chan FetcherRequest) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("redirect not supported")
		},
		// Timeout in less than 10 seconds since that is the minimum interval
		// between fetches of vehicle locations.
		// TODO Make this configurable.  OR, stop using Fetcher, and instead have
		// multiple consumers of Fetcher share a RateRegulator, so each consumer
		// can have its own http.Client, with its own timeout.
		Timeout: time.Duration(9500) * time.Millisecond,
	}
	for {
		// If we know there is a high priority request, ignore the low priority
		// channel.
		// TODO Don't look at the low priority channel if the
		ch2 := loPriorityFetchRequestChan
		if len(hiPriorityFetchRequestChan) > 0 {
			ch2 = nil
		}
		var fr FetcherRequest
		var ok bool
		isHighPriorityRequest := false
		select {
		case fr, ok = <-hiPriorityFetchRequestChan:
			if ok {
				isHighPriorityRequest = true
				break
			}
			if loPriorityFetchRequestChan == nil {
				// Both channels have been closed.
				return
			}
			hiPriorityFetchRequestChan = nil
			continue
		case fr, ok = <-ch2:
			if ok {
				break
			}
			if hiPriorityFetchRequestChan == nil {
				// Both channels have been closed.
				return
			}
			loPriorityFetchRequestChan = nil
			continue
		}
		request := fr.Request()
		if request == nil {
			// NOT calling fr.Result if fr.Request didn't provide a request;
			// it is up to fr.Request to determine whether it needs to call
			// fr.Result (or the equivalent).
			continue
		}
		if isHighPriorityRequest { /* do something different */
		}

		// Not sure that I need to add this, as the default transport probably
		// already does this.
		request.Header.Add("Connection", "keep-alive")

		glog.V(2).Infof("Executing http.Request: %+v", request)

		response, responseErr := client.Do(request)
		if responseErr != nil && glog.V(1) {
			glog.Infof("Error executing http.Client.Do\nURL: %s\nError: %s",
				request.URL, responseErr)
		}
		var body []byte
		var bodyErr error
		if response != nil {
			if response.Body != nil {
				body, bodyErr = ioutil.ReadAll(response.Body)
				response.Body.Close()
				if bodyErr != nil {
					if len(body) > 10000 {
						body = body[0:4096]
					}
					glog.Infof(
						"Error reading response body.\nURL: %s\nError: %s\nBody: %q",
						request.URL, bodyErr, body)
					body = nil // Can't trust it.
				}
			}
			if response.StatusCode != http.StatusOK {
				glog.V(1).Infof("Unusual response status: %s\nURL: %s",
					response.Status, request.URL)
			} else if responseErr == nil && bodyErr == nil {
				response, responseErr, body, bodyErr = maybeInjectFetchFailure(response, body)
			}
		}

		if glog.V(2) {
			glog.Infof("   response: %+v", response)
			glog.Infof("responseErr: %+v", responseErr)
			glog.Infof("  len(body): %d", len(body))
			glog.Infof("    bodyErr: %+v", bodyErr)
		}

		fr.Result(response, responseErr, body, bodyErr)

		// TODO Estimate the total size of the response, and then determine the
		// rate at which we've been getting data from the server. NextBus documents
		// a limit of 2MB/20sec.  If we're going too fast, wait a bit.  Or maybe
		// better, come up with a way to modify what ioutil.ReadAll does so that
		// we consume at that rate.

		//		estimatedSize := 1024 /*request overhead*/ +

	}
}

type HeaderItem struct {
	Key    string
	Values []string
}

type HeaderItems []HeaderItem

func (p HeaderItems) Len() int           { return len(p) }
func (p HeaderItems) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p HeaderItems) Less(i, j int) bool { return p[i].Key < p[j].Key }

func SortHeaderItems(header http.Header) HeaderItems {
	result := make(HeaderItems, 0, len(header))
	for key, values := range header {
		result = append(result, HeaderItem{key, values})
	}
	sort.Sort(result)
	return result
}

func EstimateRequestSize(request *http.Request) (uint64, error) {
	var w CountingBitBucketWriter
	err := request.Write(&w)
	return w.Size(), err
}

func GetContentType(response *http.Response, body []byte) string {
	var contentType string
	if response != nil && response.Header != nil {
		contentType = response.Header.Get("Content-Type")
	}
	if strings.TrimSpace(contentType) == "" && len(body) > 0 {
		contentType = http.DetectContentType(body)
	}
	return contentType
}

func ContentTypeIsXml(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return (contentType == "text/xml" ||
		strings.HasPrefix(contentType, "text/xml;"))
}

func BodyIsXml(response *http.Response, body []byte) bool {
	if response == nil || len(body) == 0 {
		return false
	}
	contentType := GetContentType(response, body)
	return ContentTypeIsXml(contentType)
}

func ContentTypeIsHtml(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return (contentType == "text/html" ||
		strings.HasPrefix(contentType, "text/html;"))
}

func BodyIsHtml(response *http.Response, body []byte) bool {
	if response == nil || len(body) == 0 {
		return false
	}
	contentType := GetContentType(response, body)
	return ContentTypeIsHtml(contentType)
}

func GetServerTime(response *http.Response) (serverTime time.Time, found bool) {
	found = false
	serverTimeStr := response.Header.Get("Date")
	if serverTimeStr != "" {
		var err error
		serverTime, err = http.ParseTime(serverTimeStr)
		if err == nil {
			found = true
		}
	}
	return
}

func NewRateRegulatedTransport(regulator RateRegulator) *http.Transport {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	dialFn := func(network, addr string) (net.Conn, error) {
		conn, err := baseDialer.Dial(network, addr)
		if conn != nil {
			conn = NewRateRegulatedConn(conn, regulator)
		}
		return conn, err
	}
	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                dialFn,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true,
		DisableKeepAlives:   true,
	}
}

func NewClientAndTransport() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{
		Transport: transport,
	}
}
