package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen is returned when circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when circuit breaker is half-open and max requests exceeded
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// State represents circuit breaker state
type State int

const (
	// StateClosed allows all requests
	StateClosed State = iota

	// StateOpen blocks all requests
	StateOpen

	// StateHalfOpen allows limited requests to test recovery
	StateHalfOpen
)

// String returns string representation of state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Settings configures circuit breaker behavior
type Settings struct {
	// MaxRequests is the maximum number of requests allowed in half-open state
	MaxRequests uint32

	// Interval is the cyclic period in closed state to clear internal counts
	Interval time.Duration

	// Timeout is the period in open state before switching to half-open
	Timeout time.Duration

	// ReadyToTrip determines if circuit should trip to open state
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called when state changes
	OnStateChange func(name string, from State, to State)
}

// Counts holds circuit breaker statistics
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
	name          string
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	readyToTrip   func(counts Counts) bool
	onStateChange func(name string, from State, to State)

	mu         sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, settings Settings) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:          name,
		maxRequests:   settings.MaxRequests,
		interval:      settings.Interval,
		timeout:       settings.Timeout,
		readyToTrip:   settings.ReadyToTrip,
		onStateChange: settings.OnStateChange,
	}

	// Set defaults
	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}

	if cb.interval == 0 {
		cb.interval = time.Second * 60
	}

	if cb.timeout == 0 {
		cb.timeout = time.Second * 60
	}

	if cb.readyToTrip == nil {
		cb.readyToTrip = defaultReadyToTrip
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// defaultReadyToTrip trips circuit after 5 consecutive failures
func defaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// Execute runs the given function if circuit breaker is closed or half-open
func (cb *CircuitBreaker) Execute(fn func() error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			cb.afterRequest(generation, false)
			panic(r)
		}
	}()

	err = fn()
	cb.afterRequest(generation, err == nil)

	return err
}

// beforeRequest checks if request can proceed
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrCircuitOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

// afterRequest records request result
func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess handles successful request
func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0

	case StateHalfOpen:
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0

		if cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
			cb.setState(StateClosed, now)
		}
	}
}

// onFailure handles failed request
func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.TotalFailures++
		cb.counts.ConsecutiveFailures++
		cb.counts.ConsecutiveSuccesses = 0

		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}

	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

// currentState returns current state and generation
func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}

	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}

	return cb.state, cb.generation
}

// setState changes circuit breaker state
func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

// toNewGeneration resets counts and increments generation
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}

	case StateOpen:
		cb.expiry = now.Add(cb.timeout)

	default: // StateHalfOpen
		cb.expiry = zero
	}
}

// GetState returns current state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)

	return state
}

// GetCounts returns current counts
func (cb *CircuitBreaker) GetCounts() Counts {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.counts
}

// Reset resets circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.toNewGeneration(time.Now())
	cb.state = StateClosed
}
