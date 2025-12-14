package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Backend represents a backend server
type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
	AvgLatency   int64 // in milliseconds
	RequestCount int64
	TotalLatency int64
}

// SetAlive sets the alive status of the backend
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

// IsAlive returns the alive status of the backend
func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	alive := b.Alive
	b.mux.RUnlock()
	return alive
}

// UpdateLatency updates the average latency for this backend
func (b *Backend) UpdateLatency(latency int64) {
	atomic.AddInt64(&b.TotalLatency, latency)
	atomic.AddInt64(&b.RequestCount, 1)

	count := atomic.LoadInt64(&b.RequestCount)
	total := atomic.LoadInt64(&b.TotalLatency)

	if count > 0 {
		atomic.StoreInt64(&b.AvgLatency, total/count)
	}
}

// GetAvgLatency returns the average latency
func (b *Backend) GetAvgLatency() int64 {
	return atomic.LoadInt64(&b.AvgLatency)
}

// ServerPool holds information about reachable backends
type ServerPool struct {
	backends []*Backend
	current  uint64
	mux      sync.RWMutex
}

// AddBackend adds a backend to the server pool
func (s *ServerPool) AddBackend(backend *Backend) {
	s.mux.Lock()
	s.backends = append(s.backends, backend)
	s.mux.Unlock()
}

// NextIndex atomically increases the counter and returns next index
func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, 1) % uint64(len(s.backends)))
}

// GetNextPeer returns next active peer using round-robin
func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex()
	l := len(s.backends) + next

	for i := next; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

// GetLeastLatencyPeer returns the backend with lowest average latency
func (s *ServerPool) GetLeastLatencyPeer() *Backend {
	s.mux.RLock()
	defer s.mux.RUnlock()

	var best *Backend
	var minLatency int64 = 1<<63 - 1

	for _, backend := range s.backends {
		if !backend.IsAlive() {
			continue
		}
		latency := backend.GetAvgLatency()
		if latency == 0 {
			latency = 100 // Default latency for new backends
		}
		if latency < minLatency {
			minLatency = latency
			best = backend
		}
	}
	return best
}

// HealthCheck pings backends and updates status
func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		status := "up"
		alive := isBackendAlive(b.URL)
		b.SetAlive(alive)
		if !alive {
			status = "down"
		}
		log.Printf("[Health Check] %s [%s] Avg Latency: %dms\n",
			b.URL, status, b.GetAvgLatency())
	}
}

// GetBackends returns all backends with their stats
func (s *ServerPool) GetBackends() []map[string]interface{} {
	s.mux.RLock()
	defer s.mux.RUnlock()

	result := make([]map[string]interface{}, len(s.backends))
	for i, b := range s.backends {
		result[i] = map[string]interface{}{
			"url":           b.URL.String(),
			"alive":         b.IsAlive(),
			"avg_latency":   b.GetAvgLatency(),
			"request_count": atomic.LoadInt64(&b.RequestCount),
		}
	}
	return result
}

// isBackendAlive checks if backend is alive
func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := http.Get(u.String() + "/health")
	if err != nil {
		return false
	}
	defer conn.Body.Close()
	return conn.StatusCode == 200
}

// healthCheckRoutine runs periodic health checks
func healthCheckRoutine(s *ServerPool) {
	t := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-t.C:
			log.Println("Starting health check...")
			s.HealthCheck()
		}
	}
}

var serverPool ServerPool
var useAdaptive = false

// lb load balances the incoming request
func lb(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var peer *Backend
	if useAdaptive {
		peer = serverPool.GetLeastLatencyPeer()
	} else {
		peer = serverPool.GetNextPeer()
	}

	if peer != nil {
		// Track request latency
		peer.ReverseProxy.ServeHTTP(w, r)
		latency := time.Since(start).Milliseconds()
		peer.UpdateLatency(latency)

		log.Printf("[%s] Forwarded to %s | Latency: %dms | Avg: %dms\n",
			r.Method, peer.URL, latency, peer.GetAvgLatency())
		return
	}

	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

// statsHandler returns load balancer statistics
func statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := map[string]interface{}{
		"algorithm": func() string {
			if useAdaptive {
				return "adaptive (latency-based)"
			}
			return "round-robin"
		}(),
		"backends": serverPool.GetBackends(),
	}
	json.NewEncoder(w).Encode(stats)
}

// toggleAlgorithm switches between round-robin and adaptive
func toggleAlgorithm(w http.ResponseWriter, r *http.Request) {
	useAdaptive = !useAdaptive
	algorithm := "round-robin"
	if useAdaptive {
		algorithm = "adaptive (latency-based)"
	}
	log.Printf("Switched to %s algorithm\n", algorithm)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"algorithm": algorithm,
		"message":   "Algorithm switched successfully",
	})
}

func main() {
	// Define backend servers (adjust ports as needed)
	backendURLs := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}

	// Parse backends and add to server pool
	for _, urlStr := range backendURLs {
		serverURL, err := url.Parse(urlStr)
		if err != nil {
			log.Fatal(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(serverURL)

		// Custom error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			log.Printf("[%s] %s\n", serverURL.Host, e.Error())
			retries := 3
			ctx := r.Context()

			for retries > 0 {
				select {
				case <-ctx.Done():
					http.Error(w, "Request timeout", http.StatusGatewayTimeout)
					return
				default:
					retries--
					peer := serverPool.GetNextPeer()
					if peer != nil {
						peer.ReverseProxy.ServeHTTP(w, r)
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			http.Error(w, "Service not available", http.StatusServiceUnavailable)
		}

		backend := &Backend{
			URL:          serverURL,
			Alive:        true,
			ReverseProxy: proxy,
		}
		serverPool.AddBackend(backend)
		log.Printf("Configured backend: %s\n", serverURL)
	}

	// Start health check routine
	go healthCheckRoutine(&serverPool)

	// Setup HTTP server
	server := http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Route special endpoints
			if r.URL.Path == "/lb/stats" {
				statsHandler(w, r)
				return
			}
			if r.URL.Path == "/lb/toggle" {
				toggleAlgorithm(w, r)
				return
			}
			// Default: load balance
			lb(w, r)
		}),
	}

	log.Println("Load Balancer started at :8080")
	log.Println("Available endpoints:")
	log.Println("  - http://localhost:8080/* (proxied requests)")
	log.Println("  - http://localhost:8080/lb/stats (statistics)")
	log.Println("  - http://localhost:8080/lb/toggle (switch algorithm)")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
