// Package proxypool provides a multi-proxy pool with round-robin selection and health checking.
// It replaces the single proxy-url global setting with a pool of proxy URLs that are rotated
// and health-checked automatically.
package proxypool

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

// ProxyEntry represents a single proxy with its state.
type ProxyEntry struct {
	URL          string    `json:"url"`
	Healthy      bool      `json:"healthy"`
	FailureCount int       `json:"failure_count"`
	LastCheck    time.Time `json:"last_check"`
	Latency      time.Duration `json:"latency"`

	mu sync.Mutex
}

// PoolConfig holds proxy pool configuration.
type PoolConfig struct {
	// URLs is the list of proxy URLs.
	URLs []string `yaml:"proxy-urls" json:"proxy-urls"`

	// Strategy determines the selection algorithm: "round-robin", "random", "least-failed".
	Strategy string `yaml:"strategy" json:"strategy"`

	// HealthCheckInterval in seconds. 0 disables health checking.
	HealthCheckInterval int `yaml:"health-check-interval" json:"health-check-interval"`

	// HealthCheckURL is the URL to probe for health checks.
	HealthCheckURL string `yaml:"health-check-url" json:"health-check-url"`

	// MaxFailures is the consecutive failure count before marking unhealthy.
	MaxFailures int `yaml:"max-failures" json:"max-failures"`
}

// ProxyPool manages a pool of proxy URLs with health tracking.
type ProxyPool struct {
	entries  []*ProxyEntry
	strategy string
	maxFails int

	index   uint64 // atomic counter for round-robin
	mu      sync.RWMutex
	stopped chan struct{}
}

var (
	defaultPool     *ProxyPool
	defaultPoolOnce sync.Once
	defaultPoolMu   sync.Mutex
)

// DefaultPool returns the global proxy pool singleton.
// Returns nil if the pool has not been initialized.
func DefaultPool() *ProxyPool {
	defaultPoolMu.Lock()
	defer defaultPoolMu.Unlock()
	return defaultPool
}

// InitPool creates or replaces the global proxy pool from configuration.
// If urls is empty, the pool is set to nil (disabled).
func InitPool(cfg PoolConfig) *ProxyPool {
	defaultPoolMu.Lock()
	defer defaultPoolMu.Unlock()

	// Stop existing pool if any
	if defaultPool != nil {
		defaultPool.Stop()
	}

	if len(cfg.URLs) == 0 {
		defaultPool = nil
		log.Info("[ProxyPool] No proxy-urls configured, proxy pool disabled")
		return nil
	}

	strategy := cfg.Strategy
	if strategy == "" {
		strategy = "round-robin"
	}

	maxFails := cfg.MaxFailures
	if maxFails <= 0 {
		maxFails = 3
	}

	entries := make([]*ProxyEntry, len(cfg.URLs))
	for i, u := range cfg.URLs {
		entries[i] = &ProxyEntry{
			URL:     u,
			Healthy: true, // initially assume all healthy
		}
	}

	pool := &ProxyPool{
		entries:  entries,
		strategy: strategy,
		maxFails: maxFails,
		stopped:  make(chan struct{}),
	}

	defaultPool = pool
	log.WithFields(log.Fields{
		"count":    len(entries),
		"strategy": strategy,
	}).Info("[ProxyPool] Proxy pool initialized")

	return pool
}

// Next returns the next available proxy URL based on the configured strategy.
// Returns empty string if no healthy proxies are available.
func (p *ProxyPool) Next() string {
	if p == nil || len(p.entries) == 0 {
		return ""
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	switch p.strategy {
	case "random":
		return p.nextRandom()
	case "least-failed":
		return p.nextLeastFailed()
	default: // round-robin
		return p.nextRoundRobin()
	}
}

// nextRoundRobin selects the next healthy proxy in sequence.
func (p *ProxyPool) nextRoundRobin() string {
	n := len(p.entries)
	start := atomic.AddUint64(&p.index, 1) - 1

	// Try all entries starting from current index
	for i := 0; i < n; i++ {
		idx := int((start + uint64(i)) % uint64(n))
		entry := p.entries[idx]
		entry.mu.Lock()
		healthy := entry.Healthy
		entry.mu.Unlock()
		if healthy {
			return entry.URL
		}
	}

	// All unhealthy: fall back to first entry (best effort)
	log.Warn("[ProxyPool] All proxies unhealthy, using first entry as fallback")
	return p.entries[0].URL
}

// nextRandom selects a random healthy proxy.
func (p *ProxyPool) nextRandom() string {
	healthy := p.healthyEntries()
	if len(healthy) == 0 {
		log.Warn("[ProxyPool] All proxies unhealthy, using random fallback")
		return p.entries[rand.Intn(len(p.entries))].URL
	}
	return healthy[rand.Intn(len(healthy))].URL
}

// nextLeastFailed selects the healthy proxy with the fewest failures.
func (p *ProxyPool) nextLeastFailed() string {
	healthy := p.healthyEntries()
	if len(healthy) == 0 {
		log.Warn("[ProxyPool] All proxies unhealthy, using least-failed fallback")
		healthy = p.entries // use all entries
	}

	best := healthy[0]
	best.mu.Lock()
	bestFails := best.FailureCount
	best.mu.Unlock()

	for _, e := range healthy[1:] {
		e.mu.Lock()
		fails := e.FailureCount
		e.mu.Unlock()
		if fails < bestFails {
			best = e
			bestFails = fails
		}
	}
	return best.URL
}

// healthyEntries returns all entries currently marked as healthy.
func (p *ProxyPool) healthyEntries() []*ProxyEntry {
	result := make([]*ProxyEntry, 0, len(p.entries))
	for _, e := range p.entries {
		e.mu.Lock()
		if e.Healthy {
			result = append(result, e)
		}
		e.mu.Unlock()
	}
	return result
}

// MarkFailed records a failure for the proxy with the given URL.
func (p *ProxyPool) MarkFailed(url string) {
	if p == nil {
		return
	}
	for _, e := range p.entries {
		if e.URL == url {
			e.mu.Lock()
			e.FailureCount++
			if e.FailureCount >= p.maxFails {
				if e.Healthy {
					log.WithField("proxy", url).Warnf("[ProxyPool] Proxy marked unhealthy after %d failures", e.FailureCount)
				}
				e.Healthy = false
			}
			e.mu.Unlock()
			return
		}
	}
}

// MarkSuccess resets the failure count and marks the proxy as healthy.
func (p *ProxyPool) MarkSuccess(url string) {
	if p == nil {
		return
	}
	for _, e := range p.entries {
		if e.URL == url {
			e.mu.Lock()
			wasUnhealthy := !e.Healthy
			e.FailureCount = 0
			e.Healthy = true
			e.mu.Unlock()
			if wasUnhealthy {
				log.WithField("proxy", url).Info("[ProxyPool] Proxy recovered, marked healthy")
			}
			return
		}
	}
}

// Entries returns a snapshot of all proxy entries (for management API / dashboard).
func (p *ProxyPool) Entries() []ProxyEntry {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]ProxyEntry, len(p.entries))
	for i, e := range p.entries {
		e.mu.Lock()
		result[i] = ProxyEntry{
			URL:          e.URL,
			Healthy:      e.Healthy,
			FailureCount: e.FailureCount,
			LastCheck:    e.LastCheck,
			Latency:      e.Latency,
		}
		e.mu.Unlock()
	}
	return result
}

// Reload replaces the pool entries with new URLs, preserving health state for URLs that remain.
func (p *ProxyPool) Reload(urls []string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	// Build lookup of existing entries
	existing := make(map[string]*ProxyEntry)
	for _, e := range p.entries {
		existing[e.URL] = e
	}

	newEntries := make([]*ProxyEntry, len(urls))
	for i, u := range urls {
		if e, ok := existing[u]; ok {
			newEntries[i] = e // preserve state
		} else {
			newEntries[i] = &ProxyEntry{URL: u, Healthy: true}
		}
	}

	p.entries = newEntries
	atomic.StoreUint64(&p.index, 0)

	log.WithField("count", len(newEntries)).Info("[ProxyPool] Pool reloaded")
}

// Stop signals the pool to stop any background goroutines.
func (p *ProxyPool) Stop() {
	if p == nil {
		return
	}
	select {
	case <-p.stopped:
		// already stopped
	default:
		close(p.stopped)
	}
}

// Stopped returns the stop channel for the pool.
func (p *ProxyPool) Stopped() <-chan struct{} {
	return p.stopped
}

// Size returns the number of proxies in the pool.
func (p *ProxyPool) Size() int {
	if p == nil {
		return 0
	}
	return len(p.entries)
}
