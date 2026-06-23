package mcp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openagenthub/backend/internal/models"
)

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	b := newCircuitBreaker(3, time.Minute)

	// Allow requests before threshold
	for i := 0; i < 2; i++ {
		if !b.Allow() {
			t.Fatalf("should allow before failure #%d", i)
		}
		b.onFailure()
	}
	// 3rd failure reaches threshold -> open
	if !b.Allow() {
		t.Fatal("should still allow before reaching threshold")
	}
	b.onFailure()
	if b.Allow() {
		t.Fatal("should trip after reaching threshold (fast-fail)")
	}
}

func TestCircuitBreaker_HalfOpenRecoversOnSuccess(t *testing.T) {
	b := newCircuitBreaker(1, 20*time.Millisecond)

	b.onFailure() // threshold=1, immediately open
	if b.Allow() {
		t.Fatal("should be in open state")
	}

	time.Sleep(30 * time.Millisecond) // after cooldown
	if !b.Allow() {
		t.Fatal("should enter half-open and allow one probe after cooldown")
	}
	b.onSuccess() // probe succeeded -> close
	if !b.Allow() {
		t.Fatal("should resume closed state after successful probe")
	}
}

func TestCircuitBreaker_HalfOpenReopensOnFailure(t *testing.T) {
	b := newCircuitBreaker(1, 20*time.Millisecond)
	b.onFailure()
	time.Sleep(30 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("should allow probe after cooldown")
	}
	b.onFailure() // probe failed -> immediately reopen
	if b.Allow() {
		t.Fatal("should re-trip after half-open probe failure")
	}
}

// End-to-end: upstream keeps returning 5xx; after threshold, invokeUpstreamMCP should be short-circuited by the circuit breaker.
func TestInvokeUpstream_TripsOnRepeated5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	server := models.ConnectedMCPServer{Endpoint: srv.URL, AuthType: "none"}
	server.ID = "srv-trip-test" // unique ID -> fresh circuit breaker

	// Before threshold: each call returns a real 5xx error, not a circuit-breaker error
	for i := 0; i < defaultFailureThreshold; i++ {
		_, err := invokeUpstreamMCP(server, "x", nil)
		if err == nil {
			t.Fatalf("call #%d should fail", i)
		}
		if errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("call #%d should not be a circuit-breaker error (still accumulating)", i)
		}
	}
	// After threshold: fast-fail
	if _, err := invokeUpstreamMCP(server, "x", nil); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("should trip after reaching threshold, got %v", err)
	}
}

// 4xx should not trigger circuit breaking (peer is healthy, just rejected the request).
func TestInvokeUpstream_4xxDoesNotTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad`))
	}))
	defer srv.Close()

	server := models.ConnectedMCPServer{Endpoint: srv.URL, AuthType: "none"}
	server.ID = "srv-4xx-test"

	for i := 0; i < defaultFailureThreshold+2; i++ {
		_, err := invokeUpstreamMCP(server, "x", nil)
		if errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("4xx should not trip circuit breaker, but call #%d was tripped", i)
		}
	}
}

func TestBreakerFor_StablePerServer(t *testing.T) {
	a1 := breakerFor("srv-A")
	a2 := breakerFor("srv-A")
	bb := breakerFor("srv-B")
	if a1 != a2 {
		t.Fatal("same server should reuse the same circuit breaker")
	}
	if a1 == bb {
		t.Fatal("different servers should have independent circuit breakers")
	}
}
