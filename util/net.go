package util

import (
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"net"
	"sync/atomic"
	"time"
)

// Assuming here that a connection is used by only a single caller at a time,
// not multiple as the net.Conn documentation allows for.
type rrcHelper struct {
	name string

	// Estimate of the fraction of bytes offered/available will actually be used.
	usedFraction float32

	// How long the operation usually takes (i.e. time to actually read
	// or write); we can subtract this from the 'waitBefore' period so
	// that we don't wait too long.
	opDuration time.Duration

	// Time since the end of the last I/O call (or since this struct
	// was created if waitAfter has yet to finish for this connection).
	periodStart time.Time

	// End time of last waitBefore call (i.e. just before the Read or Write
	// operation).
	ioStart time.Time

	// True if waitBefore has been called, but not waitAfter.
	started bool

	// Minimum amount of time that we'll claim for the period (needed because the
	// system clock may not update during a period).
	_MIN_DURATION_ time.Duration

	doLog1 glog.Verbose
	doLog2 glog.Verbose
}

func (p *rrcHelper) waitBefore(bBytes int, regulator RateRegulator) {
	if p.started {
		panic("Already called waitBefore")
	}
	frac := p.usedFraction
	if frac == 0 {
		frac = 1
	}
	prediction := float32(bBytes) * frac
	if prediction > 0 {
		waitFor := regulator.MayUse(uint(prediction))
		if p.doLog2 {
			glog.Flush()
			glog.Infof("%s offered=%d  prediction=%.1f  waitFor=%s  opDuration=%s",
				p.name, bBytes, prediction, waitFor, p.opDuration)
			glog.Flush()
		}
		waitFor -= p.opDuration
		time.Sleep(waitFor)
	} else if p.doLog2 {
		glog.Flush()
		glog.Infof("%s offered=%d  prediction=%.1f  frac=%f",
			p.name, bBytes, prediction, frac)
		glog.Flush()
	}
	p.started = true
	p.ioStart = time.Now()
}

func (p *rrcHelper) waitAfter(offered, actual int, regulator RateRegulator) {
	done := time.Now()
	if !p.started {
		panic("Haven't called waitBefore")
	}

	// Total period across which we're 'amortizing' the actual bytes read or
	// written, including any time spent sleeping in waitBefore.
	periodDuration := done.Sub(p.periodStart)

	// Duration of the I/O call.
	currentOpDuration := done.Sub(p.ioStart)
	if currentOpDuration <= p._MIN_DURATION_ {
		currentOpDuration = p._MIN_DURATION_
	}
	if periodDuration < currentOpDuration {
		periodDuration = currentOpDuration
	}

	if p.doLog2 {
		glog.Flush()
		bps := float64(actual) / periodDuration.Seconds()
		glog.Infof("%s offered=%d   actual=%d   period=%s   opDur=%s  rate=%d bytes/sec",
			p.name, offered, actual, periodDuration, currentOpDuration,
			int64(bps))
		glog.Flush()
	}

	// Figure out how long to wait, and wait that long.
	if 0 <= actual && actual <= offered {
		// Adjust used fraction (weighted average, 10% current fraction,
		// 90% previous fractions).
		currentFraction := float32(actual) / float32(offered)
		oldFraction := p.usedFraction
		if p.usedFraction == 0 {
			p.usedFraction = currentFraction
		} else {
			p.usedFraction = p.usedFraction*0.9 + currentFraction*0.1
		}
		// Given how many bytes we consumed from the available pool, and how long
		// we took to do so (including the time spent waiting in waitBefore), find
		// out how long we should wait, if at all.
		waitFor := regulator.Used(uint(actual), periodDuration)
		if p.doLog2 {
			glog.Flush()
			glog.Infof("%s waitFor=%s   fractions: prev=%.2f  current=%.2f  next=%.2f",
				p.name, waitFor, oldFraction, currentFraction, p.usedFraction)
			glog.Flush()
		}
		time.Sleep(waitFor)
	} else if actual > offered {
		glog.Errorf("%s actual is greater than offered!  %d > %d",
			p.name, actual, offered)
	}

	// Adjust opDuration (weighted average, 10% current duration,
	// 90% previous durations).
	oldOpDuration := p.opDuration
	if currentOpDuration >= 0 {
		if p.opDuration == 0 {
			p.opDuration = currentOpDuration
		} else {
			p.opDuration = (p.opDuration*9 + currentOpDuration) / 10
		}
	}

	if p.doLog2 {
		glog.Flush()
		glog.Infof("%s call durations: prev=%s  curr=%s  next=%s  total=%s",
			p.name, oldOpDuration, currentOpDuration,
			p.opDuration, periodDuration)
		glog.Flush()
	}

	p.started = false
	p.periodStart = done
}

type RateRegulatedConn struct {
	net.Conn
	regulator RateRegulator

	id           int32
	openTime     time.Time
	bytesRead    int64
	bytesWritten int64
	nextReport   time.Time

	readHelper  rrcHelper
	writeHelper rrcHelper
}

var rrcLastId int32

func NewRateRegulatedConn(conn net.Conn, regulator RateRegulator) net.Conn {
	// Should we be logging? I want to do this once for perf reasons (though
	// logging itself is expensive).
	doLog1 := glog.V(1)
	doLog2 := glog.V(2)

	// A very short, but non-zero duration (i.e. enough time for a couple of
	// context switches around an I/O operation satisfied by the kernel buffers).
	minDuration := 10 * time.Microsecond

	// Giving each a unique id so that debugging is easier.
	id := atomic.AddInt32(&rrcLastId, 1)

	result := &RateRegulatedConn{
		Conn:      conn,
		regulator: regulator,
		id:        id,
		readHelper: rrcHelper{
			name:           fmt.Sprint("R", id),
			_MIN_DURATION_: minDuration,
			doLog1:         doLog1,
			doLog2:         doLog2,
		},
		writeHelper: rrcHelper{
			name:           fmt.Sprint("W", id),
			_MIN_DURATION_: minDuration,
			doLog1:         doLog1,
			doLog2:         doLog2,
		},
	}
	now := time.Now()
	result.openTime = now
	result.readHelper.periodStart = now
	result.writeHelper.periodStart = now
	result.nextReport = now.Add(1 * time.Second)
	return result
}

func (p *RateRegulatedConn) Report() {
	if !p.readHelper.doLog1 {
		return
	}
	now := time.Now()
	if !now.After(p.nextReport) {
		return
	}
	glog.Flush()
	glog.Warning(p)
	glog.Flush()
	p.nextReport = p.nextReport.Add(1 * time.Second)
}

// Reads up to len(b) bytes from the connection, with sleeping before or after
// the reading based on the availability of tokens from the RateRegulator.
func (p *RateRegulatedConn) Read(b []byte) (n int, err error) {
	offered := len(b)
	p.readHelper.waitBefore(offered, p.regulator)
	n, err = p.Conn.Read(b)
	p.readHelper.waitAfter(offered, n, p.regulator)
	p.bytesRead += int64(n)
	p.Report()
	return
}

// Writes b to the underlying connection, and waits as long as the regulator
// indicates it should.
func (p *RateRegulatedConn) Write(b []byte) (n int, err error) {
	offered := len(b)
	p.writeHelper.waitBefore(offered, p.regulator)
	n, err = p.Conn.Write(b)
	p.writeHelper.waitAfter(offered, n, p.regulator)
	p.bytesWritten += int64(n)
	p.Report()
	return
}

func (p *RateRegulatedConn) String() string {
	var buf bytes.Buffer
	openDuration := time.Since(p.openTime)
	totalBytes := p.bytesRead + p.bytesWritten
	bytesPerSec := float64(totalBytes) / openDuration.Seconds()
	fmt.Fprintf(&buf,
		"Connection %d   open=%s   r=%d   w=%d   total=%d   rate=%d bytes/sec",
		p.id, openDuration, p.bytesRead, p.bytesWritten,
		totalBytes, int64(bytesPerSec))
	return buf.String()
}

func (p *RateRegulatedConn) Close() error {
	if p.readHelper.doLog1 {
		glog.Flush()
		glog.Infof("Closing %s", p)
		glog.Flush()
	}
	return p.Conn.Close()
}
