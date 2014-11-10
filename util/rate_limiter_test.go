package util

import (
	"testing"
	"time"
)

func newStandaloneRateRegulator(capacity uint32, interval time.Duration) *rateRegulatorImpl {
	return &rateRegulatorImpl{
		capacity:           float64(capacity),
		interval:           interval,
		lastAvailable:      float64(capacity),
		addTokensPerSecond: float64(capacity) / interval.Seconds(),
	}
}

func TestRateLimiterCore(t *testing.T) {
	rr := newStandaloneRateRegulator(100, time.Duration(100)*time.Second)
	waitForSeconds := rr.core(1, 1, 1)
	if 100.0 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 0.0 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	waitForSeconds = rr.core(1, 1, 0)
	if 99 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 1 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	waitForSeconds = rr.core(2, 1, 0)
	if 97 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 2 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	waitForSeconds = rr.core(4, 1, 0)
	if 93 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 4 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	waitForSeconds = rr.core(2, 1, 2)
	if 93 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 1 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	rr.lastAvailable = 0
	waitForSeconds = rr.core(1, 1, 0)
	if -1 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 1 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	rr.lastAvailable = 100
	waitForSeconds = rr.core(1, 1, 10000)
	if 100 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 0 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}

	rr.lastAvailable = 100
	waitForSeconds = rr.core(2, 1, 10000)
	if 99 != rr.lastAvailable {
		t.Error("Wrong lastAvailable:", rr.lastAvailable)
	}
	if 1 != waitForSeconds {
		t.Error("Wrong waitForSeconds:", waitForSeconds)
	}
}

func TestXYZ(t *testing.T) {

}
