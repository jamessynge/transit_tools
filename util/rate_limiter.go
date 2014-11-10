package util

import (
	"fmt"
	"github.com/golang/glog"
//	"runtime"
	"time"
)

// Support for limiting the rate at which we make requests to NextBus (and
// other sites), or performing other activities.  Since we don't necessarily
// know the size of the response when making a request, we can instead track
// the use of the resource after the fact, and delay the next request.
// Based on token bucket and leaky bucket ideas.

type RateRegulator interface {
	// Caller plans to use up to `prediction` tokens, and wants to know how long
	// that action should take; this allows caller to decide to do some or all of
	// the waiting in advance of the action.
	MayUse(prediction uint) time.Duration

	// Caller consumed |used| tokens 'just now' over period time.Duration.
	// Returns the amount of time
	// the caller should wait if we've run out of capacity and/or if the usage
	// rate is too high for the available tokens.
	Used(used uint, period time.Duration) time.Duration

	Close()
}

type rateRegulatorRequest struct {
	// Number of tokens consumed during period.
	used float64
	// How long the consumer took to consume the tokens (e.g. the time since
	// it stopped waiting after the last call to RateRegulator.Used).
	periodSecs float64
	// Channel for returning the amount of time the consumer should wait before
	// the next consumption of tokens.
	ch chan<- time.Duration
}

type rateRegulatorImpl struct {
	// Regulator runs as a separate go routine, so we don't need mutexes.
	requestCh chan *rateRegulatorRequest

	// Number of tokens added each interval.
	capacity float64

	// Every interval time period we add up to capacity tokens to lastAvailable.
	interval time.Duration

	// capacity / interval seconds, precomputed for convenience
	addTokensPerSecond float64

	// Time when we last calculated the number of tokens available for use.
	lastTime time.Time

	// Number of tokens available at time `lastTime`.
	lastAvailable float64

	closed bool

	vlog2 glog.Verbose
}

func (p *rateRegulatorImpl) String() string {
	pct := p.lastAvailable / p.capacity * 100
	return fmt.Sprintf("avail %d (%.1f%% of %d)",
			int64(p.lastAvailable), pct, int64(p.capacity))
}

func (p *rateRegulatorImpl) MayUse(prediction uint) time.Duration {
	ch := make(chan time.Duration)
	req := &rateRegulatorRequest{
		used:       float64(prediction),
		periodSecs: 0,
		ch:         ch,
	}
	p.requestCh <- req
	return <-ch
}

func (p *rateRegulatorImpl) Used(used uint, period time.Duration) time.Duration {
	if used == 0 {
		return 0
	}
	if period <= 0 {
		// A very short, but non-zero duration.
		period = 10 * time.Microsecond
	}
	ch := make(chan time.Duration)
	req := &rateRegulatorRequest{
		used:       float64(used),
		periodSecs: period.Seconds(),
		ch:         ch,
	}
	p.requestCh <- req
	return <-ch
}

func (p *rateRegulatorImpl) Close() {
	close(p.requestCh)
}

func (p *rateRegulatorImpl) run() {
	// Since there may be an arbitrary amount of time between when the regulator
	// is created and when it is first used (i.e. application setup time),
	// set deltaSeconds to zero on the first execution of core.
	if p.lastAvailable == 0 {
		req, ok := <-p.requestCh
		if !ok {
			p.closed = true
			return
		}
		p.lastTime = time.Now()
		waitForSecs := p.core(req.used, req.periodSecs, 0)
		req.ch <- time.Duration(waitForSecs * float64(time.Second))
	}

	for {
		req, ok := <-p.requestCh
		if !ok {
			p.closed = true
			return
		}
		now := time.Now()
		deltaSeconds := now.Sub(p.lastTime).Seconds()
		p.lastTime = now
		waitForSecs := p.core(req.used, req.periodSecs, deltaSeconds)
		req.ch <- time.Duration(waitForSecs * float64(time.Second))
	}
}

// Compute how long to wait after consuming |used| units of capacity over
// |periodSecs|.  The last call to this method was made |deltaSeconds| ago.
// Returns the number of seconds the consumer should wait.
// For testability, the core of the run loop algorithm is exposed (i.e. so
// a test can provide the time (deltaSeconds since last call).
func (p *rateRegulatorImpl) core(
	used, periodSecs, deltaSeconds float64) (waitForSecs float64) {
	if p.vlog2 {
		glog.Flush()
		if periodSecs == 0 {
			glog.Infof("rr core entry: %s    may use: %d   deltaSeconds: %.6f",
								 p, int64(used), deltaSeconds)
		} else {
			glog.Infof("rr core entry: %s    used: %d   periodSecs: %.6f   deltaSeconds: %.6f",
								 p, int64(used), periodSecs, deltaSeconds)
		}
		glog.Flush()
		defer func() {
			glog.Flush()
			glog.Infof("rr core  exit: %s    waitForSecs: %.6f", p, waitForSecs)
			glog.Flush()
		}()
	}

	// If deltaSeconds is greater than periodSecs, first adjust p.lastAvailable
	// for the time before periodSecs started.
	if deltaSeconds > periodSecs {
		if periodSecs < 0 {
			periodSecs = 0
		}
		p.lastAvailable += (deltaSeconds - periodSecs) * p.addTokensPerSecond
		if p.lastAvailable > p.capacity {
			p.lastAvailable = p.capacity
		}
		deltaSeconds = periodSecs
	}

	// If periodSecs isn't positive, this is a request to find out how long a
	// caller should take to consume |used| tokens (e.g. so caller can wait
	// for a bit before taking action).
	if periodSecs <= 0 {
		// Minimum time is the time to add that many tokens.
		waitForSecs = used / p.addTokensPerSecond
		// If we don't have enough tokens, the caller should really slow down.
		if p.lastAvailable < used {
			waitForSecs += (used - p.lastAvailable) / p.addTokensPerSecond
		}
		return
	}

	// Remove the tokens used during periodSecs by the consumer.
	p.lastAvailable -= used

	// Add the tokens that are "produced" during deltaSeconds by the "source".
	// The clock may be running backward (rare occurrence, but does happen),
	// guard against that.
	// Also compute the net change in tokens over the shorter
	// of the two intervals (periodSecs and deltaSeconds).
	var deltaTokens float64
	if deltaSeconds > 0 {
		tokensAdded := p.addTokensPerSecond * deltaSeconds
		if p.lastAvailable+tokensAdded >= p.capacity {
			tokensAdded = p.capacity - p.lastAvailable
			p.lastAvailable = p.capacity
		} else {
			p.lastAvailable += tokensAdded
		}
		if deltaSeconds < periodSecs {
			tokensConsumedPerSecond := used / periodSecs
			deltaTokens = tokensAdded - tokensConsumedPerSecond*deltaSeconds
		} else { // deltaSeconds == periodSecs
			deltaTokens = tokensAdded - used
		}
	} else {
		// Clock may be running backward (rare occurrence, but does happen,
		// usually caused by bad clock sync algorithm).
		// Estimate total consumption rate based solely on rate at which this
		// consumer is using tokens.
		deltaSeconds = periodSecs
		deltaTokens = -used
	}

	// If the consumer is using tokens faster than they are added (deltaTokens
	// is negative), then consumer should wait long enough to make up the
	// deficit (effectively lowering the rate at which they're being consumed
	// to the rate at which they are added).
	if deltaTokens < 0 {
		// How many "excess" tokens were consumed during periodSecs?
		excessTokensPerSecond := -deltaTokens / deltaSeconds
		excessTokens := excessTokensPerSecond * periodSecs
		// How long will it take to add for the "source" to produce those
		// excess tokens?
		waitForSecs = excessTokens / p.addTokensPerSecond
	}

	// If we've gone too fast and used too many tokens (e.g. across multiple
	// consumers the rate is too high), then wait until it would be safe to
	// start this operation (i.e. apply the brakes, on the assumption that after
	// waiting the consumer is going to immediately perform another operation at
	// the same rate).
	if p.lastAvailable < 0 {
		// Need to wait at least until lastAvailable is zero.
		minWaitSecs := -p.lastAvailable / p.addTokensPerSecond
		// And wait for the time this operation should take.
		minWaitSecs += (used / p.addTokensPerSecond)
		// Minus the amount of time the consumer took to use these tokens.
		minWaitSecs -= periodSecs
		// Make sure the caller is told to wait at least this long.
		if waitForSecs < minWaitSecs {
			waitForSecs = minWaitSecs
		}
	}
	return
}

func NewRateRegulator(initialAvailability, maximumCapacity uint32,
	interval time.Duration) (RateRegulator, error) {
	if maximumCapacity < 1 {
		return nil, fmt.Errorf("maximumCapacity must be > 0")
	}
	if maximumCapacity < initialAvailability {
		return nil, fmt.Errorf(
			"initialAvailability (%d) must be <= maximumCapacity (%d)",
			initialAvailability, maximumCapacity)
	}
	if interval <= 0 {
		return nil, fmt.Errorf("duration must be > 0")
	}
	ch := make(chan *rateRegulatorRequest)
	rri := &rateRegulatorImpl{
		requestCh:          ch,
		capacity:           float64(maximumCapacity),
		interval:           interval,
		lastAvailable:      float64(initialAvailability),
		lastTime:           time.Now(),
		addTokensPerSecond: float64(maximumCapacity) / interval.Seconds(),
		vlog2: glog.V(2),
	}
	go rri.run()
	return rri, nil
}

type nowaitRateRegulator struct {
	rr    RateRegulator
	accum time.Duration
}

func (p *nowaitRateRegulator) MayUse(prediction uint) time.Duration {
	return 0
}
func (p *nowaitRateRegulator) Used(used uint, period time.Duration) time.Duration {
	p.accum += p.rr.Used(used, period)
	return 0
}
func (p *nowaitRateRegulator) Close() {
	p.rr = nil
}
func (p *nowaitRateRegulator) DrainAccumulator() (result time.Duration) {
	result, p.accum = p.accum, 0.0
	return
}

// Create a wrapper around a RateRegulator, telling it about tokens being used,
// but not waiting. Used for a high-priority consumer that we don't want to
// have wait.
// TODO Want a way to move the waiting from the high-priority consumer to lower
// priority consumers.
func NewNoWaitRateRegulator(rr RateRegulator) *nowaitRateRegulator {
	return &nowaitRateRegulator{rr: rr}
}
