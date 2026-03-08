package proxypool

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

const (
	defaultHealthCheckURL      = "https://chatgpt.com/favicon.ico"
	defaultHealthCheckInterval = 60 // seconds
	defaultHealthCheckTimeout  = 10 // seconds
)

// StartHealthCheck begins a background goroutine that periodically probes each proxy.
// The goroutine stops when pool.Stop() is called.
func StartHealthCheck(pool *ProxyPool, cfg PoolConfig) {
	if pool == nil || pool.Size() == 0 {
		return
	}

	interval := cfg.HealthCheckInterval
	if interval <= 0 {
		interval = defaultHealthCheckInterval
	}

	checkURL := cfg.HealthCheckURL
	if checkURL == "" {
		checkURL = defaultHealthCheckURL
	}

	log.WithFields(log.Fields{
		"interval": interval,
		"url":      checkURL,
		"proxies":  pool.Size(),
	}).Info("[ProxyPool] Starting health check")

	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-pool.Stopped():
				log.Info("[ProxyPool] Health check stopped")
				return
			case <-ticker.C:
				checkAll(pool, checkURL)
			}
		}
	}()
}

// checkAll probes every proxy in the pool and updates health state.
func checkAll(pool *ProxyPool, checkURL string) {
	pool.mu.RLock()
	entries := pool.entries
	pool.mu.RUnlock()

	for _, entry := range entries {
		entry.mu.Lock()
		proxyURL := entry.URL
		entry.mu.Unlock()

		start := time.Now()
		err := probeViaProxy(proxyURL, checkURL)
		latency := time.Since(start)

		entry.mu.Lock()
		entry.LastCheck = time.Now()
		entry.Latency = latency

		if err != nil {
			entry.FailureCount++
			if entry.FailureCount >= pool.maxFails && entry.Healthy {
				entry.Healthy = false
				log.WithFields(log.Fields{
					"proxy":    proxyURL,
					"failures": entry.FailureCount,
					"error":    err.Error(),
				}).Warn("[ProxyPool] Health check: proxy marked unhealthy")
			}
		} else {
			if !entry.Healthy {
				log.WithFields(log.Fields{
					"proxy":   proxyURL,
					"latency": latency,
				}).Info("[ProxyPool] Health check: proxy recovered")
			}
			entry.Healthy = true
			entry.FailureCount = 0
		}
		entry.mu.Unlock()
	}
}

// probeViaProxy sends an HTTP HEAD request through the given proxy to test connectivity.
func probeViaProxy(proxyURLStr, targetURL string) error {
	parsedURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return err
	}

	var transport *http.Transport

	if parsedURL.Scheme == "socks5" {
		var proxyAuth *proxy.Auth
		if parsedURL.User != nil {
			username := parsedURL.User.Username()
			password, _ := parsedURL.User.Password()
			proxyAuth = &proxy.Auth{User: username, Password: password}
		}
		dialer, errSOCKS5 := proxy.SOCKS5("tcp", parsedURL.Host, proxyAuth, proxy.Direct)
		if errSOCKS5 != nil {
			return errSOCKS5
		}
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
	} else {
		transport = &http.Transport{Proxy: http.ProxyURL(parsedURL)}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(defaultHealthCheckTimeout) * time.Second,
	}

	req, err := http.NewRequest(http.MethodHead, targetURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}
