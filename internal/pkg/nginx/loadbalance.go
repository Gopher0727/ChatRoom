package nginx

import (
	"sync"
	"sync/atomic"
)

// 对于任何通过 Nginx 的客户端请求，应该根据负载均衡策略分发到可用的 Gateway 节点
type BackendServer struct {
	ID           string
	RequestCount int64
	Available    bool
	mu           sync.RWMutex
}

func (s *BackendServer) IncrementRequests() {
	atomic.AddInt64(&s.RequestCount, 1)
}

func (s *BackendServer) GetRequestCount() int64 {
	return atomic.LoadInt64(&s.RequestCount)
}

func (s *BackendServer) SetAvailable(available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Available = available
}

func (s *BackendServer) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Available
}

type LoadBalancer struct {
	backends []*BackendServer
	strategy string // "round_robin" or "least_connections"
	current  uint64
	mu       sync.RWMutex
}

func NewLoadBalancer(backends []*BackendServer, strategy string) *LoadBalancer {
	return &LoadBalancer{
		backends: backends,
		strategy: strategy,
		current:  0,
	}
}

// GetNextBackend returns the next backend server based on the load balancing strategy
func (lb *LoadBalancer) GetNextBackend() *BackendServer {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.backends) == 0 {
		return nil
	}

	switch lb.strategy {
	case "least_connections":
		return func() *BackendServer {
			var selected *BackendServer
			minCount := int64(-1)
			for _, backend := range lb.backends {
				if !backend.IsAvailable() {
					continue
				}
				count := backend.GetRequestCount()
				if minCount == -1 || count < minCount {
					minCount = count
					selected = backend
				}
			}
			return selected
		}()
	default: // case "round_robin"
		return func() *BackendServer {
			attempts := 0
			for attempts < len(lb.backends) {
				idx := atomic.AddUint64(&lb.current, 1) % uint64(len(lb.backends))
				backend := lb.backends[idx]
				if backend.IsAvailable() {
					return backend
				}
				attempts++
			}
			return nil
		}()
	}
}
