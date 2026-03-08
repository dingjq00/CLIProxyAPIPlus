package proxypool

import (
	"testing"
)

func TestRoundRobin_Basic(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:     []string{"http://proxy1:8080", "http://proxy2:8080", "http://proxy3:8080"},
		Strategy: "round-robin",
	})
	defer pool.Stop()

	seen := make(map[string]int)
	for i := 0; i < 9; i++ {
		url := pool.Next()
		seen[url]++
	}

	// Each should be selected 3 times
	for _, url := range []string{"http://proxy1:8080", "http://proxy2:8080", "http://proxy3:8080"} {
		if seen[url] != 3 {
			t.Errorf("expected %s to be selected 3 times, got %d", url, seen[url])
		}
	}
}

func TestRoundRobin_SkipsUnhealthy(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:     []string{"http://proxy1:8080", "http://proxy2:8080", "http://proxy3:8080"},
		Strategy: "round-robin",
	})
	defer pool.Stop()

	// Mark proxy2 as unhealthy
	pool.entries[1].Healthy = false

	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		url := pool.Next()
		seen[url]++
	}

	if seen["http://proxy2:8080"] != 0 {
		t.Error("unhealthy proxy2 should not be selected")
	}
	if seen["http://proxy1:8080"] < 1 || seen["http://proxy3:8080"] < 1 {
		t.Error("healthy proxies should be selected")
	}
}

func TestRoundRobin_AllUnhealthyFallback(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:     []string{"http://proxy1:8080", "http://proxy2:8080"},
		Strategy: "round-robin",
	})
	defer pool.Stop()

	// Mark all unhealthy
	for _, e := range pool.entries {
		e.Healthy = false
	}

	url := pool.Next()
	if url != "http://proxy1:8080" {
		t.Errorf("expected fallback to first entry, got %s", url)
	}
}

func TestMarkFailed_MarksUnhealthy(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:        []string{"http://proxy1:8080"},
		Strategy:    "round-robin",
		MaxFailures: 2,
	})
	defer pool.Stop()

	pool.MarkFailed("http://proxy1:8080")
	if !pool.entries[0].Healthy {
		t.Error("should still be healthy after 1 failure")
	}

	pool.MarkFailed("http://proxy1:8080")
	if pool.entries[0].Healthy {
		t.Error("should be unhealthy after 2 failures (max)")
	}
}

func TestMarkSuccess_Recovers(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:        []string{"http://proxy1:8080"},
		Strategy:    "round-robin",
		MaxFailures: 1,
	})
	defer pool.Stop()

	pool.MarkFailed("http://proxy1:8080")
	if pool.entries[0].Healthy {
		t.Error("should be unhealthy")
	}

	pool.MarkSuccess("http://proxy1:8080")
	if !pool.entries[0].Healthy {
		t.Error("should recover after MarkSuccess")
	}
	if pool.entries[0].FailureCount != 0 {
		t.Error("failure count should be reset")
	}
}

func TestReload_PreservesState(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:        []string{"http://proxy1:8080", "http://proxy2:8080"},
		Strategy:    "round-robin",
		MaxFailures: 1,
	})
	defer pool.Stop()

	// Mark proxy1 as unhealthy
	pool.MarkFailed("http://proxy1:8080")

	// Reload with proxy1 still present + new proxy3
	pool.Reload([]string{"http://proxy1:8080", "http://proxy3:8080"})

	if pool.Size() != 2 {
		t.Errorf("expected 2 entries, got %d", pool.Size())
	}

	// proxy1 should still be unhealthy
	pool.entries[0].mu.Lock()
	healthy1 := pool.entries[0].Healthy
	pool.entries[0].mu.Unlock()
	if healthy1 {
		t.Error("proxy1 health state should be preserved (unhealthy)")
	}

	// proxy3 should be healthy (new)
	pool.entries[1].mu.Lock()
	healthy3 := pool.entries[1].Healthy
	pool.entries[1].mu.Unlock()
	if !healthy3 {
		t.Error("new proxy3 should be healthy")
	}
}

func TestNilPool_NoPanic(t *testing.T) {
	var pool *ProxyPool
	if pool.Next() != "" {
		t.Error("nil pool should return empty string")
	}
	pool.MarkFailed("http://any")
	pool.MarkSuccess("http://any")
	// Should not panic
}

func TestRandom_Strategy(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:     []string{"http://proxy1:8080", "http://proxy2:8080"},
		Strategy: "random",
	})
	defer pool.Stop()

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		seen[pool.Next()] = true
	}

	if len(seen) < 1 {
		t.Error("random strategy should select at least one proxy")
	}
}

func TestLeastFailed_Strategy(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:        []string{"http://proxy1:8080", "http://proxy2:8080"},
		Strategy:    "least-failed",
		MaxFailures: 10, // high max so nothing goes unhealthy
	})
	defer pool.Stop()

	// Add failures to proxy1
	pool.MarkFailed("http://proxy1:8080")
	pool.MarkFailed("http://proxy1:8080")

	url := pool.Next()
	if url != "http://proxy2:8080" {
		t.Errorf("expected least-failed to select proxy2, got %s", url)
	}
}

func TestEntries_Snapshot(t *testing.T) {
	pool := InitPool(PoolConfig{
		URLs:     []string{"http://proxy1:8080"},
		Strategy: "round-robin",
	})
	defer pool.Stop()

	entries := pool.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URL != "http://proxy1:8080" {
		t.Errorf("expected URL http://proxy1:8080, got %s", entries[0].URL)
	}
	if !entries[0].Healthy {
		t.Error("expected entry to be healthy")
	}
}
