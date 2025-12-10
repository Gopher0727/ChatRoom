package nginx

import (
	"fmt"
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/Gopher0727/ChatRoom/utils"
)

// SimulateRequest simulates a request being processed by the load balancer
func (lb *LoadBalancer) SimulateRequest() bool {
	backend := lb.GetNextBackend()
	if backend == nil {
		return false
	}
	backend.IncrementRequests()
	return true
}

// TestProperty_LoadBalancerDistribution tests that load balancer distributes requests
func TestProperty_LoadBalancerDistribution(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Round-robin distributes requests evenly across all available
	properties.Property("Round-robin distributes requests evenly", prop.ForAll(
		func(numRequests int) bool {
			backends := []*BackendServer{
				{ID: "backend1", Available: true},
				{ID: "backend2", Available: true},
				{ID: "backend3", Available: true},
			}

			lb := NewLoadBalancer(backends, "round_robin")

			for range numRequests {
				if !lb.SimulateRequest() {
					return false
				}
			}

			// Check distribution
			// For round-robin, each backend should receive approximately equal requests
			expectedPerBackend := numRequests / len(backends)
			tolerance := 2 // Allow difference of 2 requests
			for _, backend := range backends {
				count := int(backend.GetRequestCount())
				diff := utils.Abs(count - expectedPerBackend)
				if diff > tolerance {
					t.Logf("Backend %s received %d requests, expected ~%d (diff: %d)",
						backend.ID, count, expectedPerBackend, diff)
					return false
				}
			}
			return true
		},
		gen.IntRange(10, 100),
	))

	// Property 2: All requests are distributed to available backends only
	properties.Property("Requests go to available backends only", prop.ForAll(
		func(numRequests int) bool {
			backends := []*BackendServer{
				{ID: "backend1", Available: true},
				{ID: "backend2", Available: false}, // This one is down
				{ID: "backend3", Available: true},
			}

			lb := NewLoadBalancer(backends, "round_robin")

			successCount := 0
			for range numRequests {
				if lb.SimulateRequest() {
					successCount++
				}
			}

			// Unavailable backend should receive no requests
			if backends[1].GetRequestCount() != 0 {
				t.Logf("Unavailable backend received %d requests", backends[1].GetRequestCount())
				return false
			}

			// Available backends should have received all requests
			totalRequests := backends[0].GetRequestCount() + backends[2].GetRequestCount()
			if totalRequests != int64(successCount) {
				t.Logf("Total requests to available backends: %d, expected: %d",
					totalRequests, successCount)
				return false
			}

			return successCount == numRequests
		},
		gen.IntRange(10, 100),
	))

	// Property 3: Least connections strategy sends requests to backend with fewest connections
	properties.Property("least_connections sends to backend with fewest connections", prop.ForAll(
		func(initialCounts []int) bool {
			if len(initialCounts) < 2 {
				return true // Skip if not enough backends
			}

			// Create backends with different initial request counts
			backends := make([]*BackendServer, len(initialCounts))
			for i, count := range initialCounts {
				backends[i] = &BackendServer{
					ID:           fmt.Sprintf("backend%d", i),
					Available:    true,
					RequestCount: int64(count),
				}
			}

			lb := NewLoadBalancer(backends, "least_connections")

			// Find the minimum connection count before selection
			minCount := backends[0].GetRequestCount()
			for _, backend := range backends {
				if backend.GetRequestCount() < minCount {
					minCount = backend.GetRequestCount()
				}
			}

			// Get next backend - should be one with least connections
			selected := lb.GetNextBackend()
			if selected == nil {
				return false
			}

			// The selected backend should have the minimum count
			selectedCount := selected.GetRequestCount()
			return selectedCount == minCount
		},
		gen.SliceOfN(5, gen.IntRange(0, 100)), // 5 backends with 0-100 initial requests
	))

	// Property 4: Load balancer handles concurrent requests correctly
	properties.Property("handles concurrent requests without race conditions", prop.ForAll(
		func(numGoroutines int, requestsPerGoroutine int) bool {
			backends := []*BackendServer{
				{ID: "backend1", Available: true},
				{ID: "backend2", Available: true},
				{ID: "backend3", Available: true},
			}

			lb := NewLoadBalancer(backends, "round-robin")

			totalRequests := numGoroutines * requestsPerGoroutine

			var wg sync.WaitGroup
			for range numGoroutines {
				wg.Go(func() {
					for range requestsPerGoroutine {
						lb.SimulateRequest()
					}
				})
			}
			wg.Wait()

			// Verify total request count
			total := int64(0)
			for _, backend := range backends {
				total += backend.GetRequestCount()
			}
			if total != int64(totalRequests) {
				t.Logf("Total requests: %d, expected: %d", total, totalRequests)
				return false
			}
			return true
		},
		gen.IntRange(2, 10), // 2-10 goroutines
		gen.IntRange(5, 20), // 5-20 requests per goroutine
	))

	// Property 5: When a backend becomes unavailable, requests are redistributed
	properties.Property("redistributes when backend becomes unavailable", prop.ForAll(
		func(numRequests int) bool {
			backends := []*BackendServer{
				{ID: "backend1", Available: true},
				{ID: "backend2", Available: true},
				{ID: "backend3", Available: true},
			}

			lb := NewLoadBalancer(backends, "round-robin")

			// Send half the requests
			halfRequests := numRequests / 2
			for range halfRequests {
				lb.SimulateRequest()
			}

			// Mark one backend as unavailable
			backends[1].SetAvailable(false)
			countBeforeFailure := backends[1].GetRequestCount()

			// Send remaining requests
			for range numRequests - halfRequests {
				lb.SimulateRequest()
			}

			// Failed backend should not receive any more requests
			if backends[1].GetRequestCount() != countBeforeFailure {
				t.Logf("Failed backend received requests after becoming unavailable")
				return false
			}

			// Other backends should have received the remaining requests
			totalOthers := backends[0].GetRequestCount() + backends[2].GetRequestCount()
			expectedTotal := int64(numRequests - int(countBeforeFailure))
			if totalOthers != expectedTotal {
				t.Logf("Other backends received %d requests, expected %d",
					totalOthers, expectedTotal)
				return false
			}

			return true
		},
		gen.IntRange(20, 100),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestLoadBalancerBasicFunctionality tests basic load balancer functionality
func TestLoadBalancerBasicFunctionality(t *testing.T) {
	t.Run("round-robin with single backend", func(t *testing.T) {
		backends := []*BackendServer{
			{ID: "backend1", Available: true},
		}

		lb := NewLoadBalancer(backends, "round-robin")

		for range 10 {
			backend := lb.GetNextBackend()
			if backend == nil || backend.ID != "backend1" {
				t.Errorf("Expected backend1, got %v", backend)
			}
		}
	})

	t.Run("no available backends", func(t *testing.T) {
		backends := []*BackendServer{
			{ID: "backend1", Available: false},
			{ID: "backend2", Available: false},
		}

		lb := NewLoadBalancer(backends, "round-robin")
		backend := lb.GetNextBackend()

		if backend != nil {
			t.Errorf("Expected nil backend, got %v", backend)
		}
	})

	t.Run("least_connections with equal connections", func(t *testing.T) {
		backends := []*BackendServer{
			{ID: "backend1", Available: true, RequestCount: 5},
			{ID: "backend2", Available: true, RequestCount: 5},
		}

		lb := NewLoadBalancer(backends, "least_connections")
		backend := lb.GetNextBackend()

		if backend == nil {
			t.Error("Expected a backend, got nil")
		}
	})

	t.Run("least_connections selects backend with fewer connections", func(t *testing.T) {
		backends := []*BackendServer{
			{ID: "backend1", Available: true, RequestCount: 10},
			{ID: "backend2", Available: true, RequestCount: 5},
			{ID: "backend3", Available: true, RequestCount: 15},
		}

		lb := NewLoadBalancer(backends, "least_connections")
		backend := lb.GetNextBackend()

		if backend == nil || backend.ID != "backend2" {
			t.Errorf("Expected backend2 (fewest connections), got %v", backend)
		}
	})
}

// TestConcurrentLoadBalancing tests concurrent access to load balancer
func TestConcurrentLoadBalancing(t *testing.T) {
	backends := []*BackendServer{
		{ID: "backend1", Available: true},
		{ID: "backend2", Available: true},
		{ID: "backend3", Available: true},
	}

	lb := NewLoadBalancer(backends, "round-robin")

	numGoroutines := 10
	requestsPerGoroutine := 20

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			for range requestsPerGoroutine {
				lb.SimulateRequest()
			}
		})
	}
	wg.Wait()

	// Verify total requests
	total := int64(0)
	for _, backend := range backends {
		total += backend.GetRequestCount()
		t.Logf("Backend %s: %d requests", backend.ID, backend.GetRequestCount())
	}

	expectedTotal := int64(numGoroutines * requestsPerGoroutine)
	if total != expectedTotal {
		t.Errorf("Total requests: %d, expected: %d", total, expectedTotal)
	}
}
