package mcp

import (
	"errors"
	"sync"
	"time"
)

// Lightweight circuit breaker: protects calls to external MCP Servers,
// avoiding repeated calls to a persistently failing upstream.
// State machine: closed (normal) -> consecutive failures reach threshold -> open (fail-fast)
// -> after cooldown, half-open (allow one probe) -> success resets to closed,
// failure re-opens. No third-party dependencies; easy to unit-test offline.

// ErrCircuitOpen indicates the upstream circuit breaker is open; calls are fast-rejected.
var ErrCircuitOpen = errors.New("upstream circuit breaker open")

type breakerState int

const (
	stateClosed breakerState = iota
	stateOpen
	stateHalfOpen
)

const (
	defaultFailureThreshold = 5
	defaultCooldown         = 30 * time.Second
)

type circuitBreaker struct {
	mu               sync.Mutex
	state            breakerState
	failures         int
	failureThreshold int
	cooldown         time.Duration
	openUntil        time.Time
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:            stateClosed,
		failureThreshold: threshold,
		cooldown:         cooldown,
	}
}

// Allow decides whether this call is allowed. When open and cooldown has passed,
// transitions to half-open and allows one probe.
func (b *circuitBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == stateOpen {
		if time.Now().After(b.openUntil) {
			b.state = stateHalfOpen
			return true
		}
		return false
	}
	return true
}

func (b *circuitBreaker) onSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = stateClosed
}

func (b *circuitBreaker) onFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	// Failures under half-open immediately re-open; under closed, accumulate until threshold.
	if b.state == stateHalfOpen || b.failures >= b.failureThreshold {
		b.state = stateOpen
		b.openUntil = time.Now().Add(b.cooldown)
	}
}

// One circuit breaker per connected server.
var (
	breakerMu sync.Mutex
	breakers  = map[string]*circuitBreaker{}
)

func breakerFor(serverID string) *circuitBreaker {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	b, ok := breakers[serverID]
	if !ok {
		b = newCircuitBreaker(defaultFailureThreshold, defaultCooldown)
		breakers[serverID] = b
	}
	return b
}
