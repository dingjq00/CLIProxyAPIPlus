package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/proxypool"
	log "github.com/sirupsen/logrus"
)

// proxyPoolResponse is the JSON payload for GET /proxy-pool.
type proxyPoolResponse struct {
	Enabled  bool                `json:"enabled"`
	Strategy string              `json:"strategy"`
	Config   proxyPoolConfigResp `json:"config"`
	Entries  []proxyEntryResp    `json:"entries"`
}

type proxyPoolConfigResp struct {
	Strategy            string `json:"strategy"`
	HealthCheckInterval int    `json:"health_check_interval"`
	HealthCheckURL      string `json:"health_check_url"`
	MaxFailures         int    `json:"max_failures"`
}

type proxyEntryResp struct {
	URL          string `json:"url"`
	Healthy      bool   `json:"healthy"`
	FailureCount int    `json:"failure_count"`
	LastCheck    string `json:"last_check"`
	Latency      string `json:"latency"`
}

// GetProxyPool returns the current proxy pool status and configuration.
func (h *Handler) GetProxyPool(c *gin.Context) {
	pool := proxypool.DefaultPool()

	resp := proxyPoolResponse{
		Enabled: pool != nil && pool.Size() > 0,
		Config: proxyPoolConfigResp{
			Strategy:            h.cfg.ProxyPoolConfig.Strategy,
			HealthCheckInterval: h.cfg.ProxyPoolConfig.HealthCheckInterval,
			HealthCheckURL:      h.cfg.ProxyPoolConfig.HealthCheckURL,
			MaxFailures:         h.cfg.ProxyPoolConfig.MaxFailures,
		},
	}

	if pool != nil {
		entries := pool.Entries()
		resp.Entries = make([]proxyEntryResp, len(entries))
		for i, e := range entries {
			lastCheck := ""
			if !e.LastCheck.IsZero() {
				lastCheck = e.LastCheck.Format("2006-01-02 15:04:05")
			}
			latency := ""
			if e.Latency > 0 {
				latency = e.Latency.String()
			}
			resp.Entries[i] = proxyEntryResp{
				URL:          e.URL,
				Healthy:      e.Healthy,
				FailureCount: e.FailureCount,
				LastCheck:    lastCheck,
				Latency:      latency,
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// proxyPoolUpdateRequest is the JSON payload for PUT /proxy-pool.
type proxyPoolUpdateRequest struct {
	URLs                []string `json:"urls"`
	Strategy            string   `json:"strategy,omitempty"`
	HealthCheckInterval int      `json:"health_check_interval,omitempty"`
	HealthCheckURL      string   `json:"health_check_url,omitempty"`
	MaxFailures         int      `json:"max_failures,omitempty"`
}

// PutProxyPool updates the proxy pool configuration and hot-reloads.
func (h *Handler) PutProxyPool(c *gin.Context) {
	var req proxyPoolUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "message": err.Error()})
		return
	}

	// Update config
	h.cfg.ProxyURLs = req.URLs
	if req.Strategy != "" {
		h.cfg.ProxyPoolConfig.Strategy = req.Strategy
	}
	if req.HealthCheckInterval > 0 {
		h.cfg.ProxyPoolConfig.HealthCheckInterval = req.HealthCheckInterval
	}
	if req.HealthCheckURL != "" {
		h.cfg.ProxyPoolConfig.HealthCheckURL = req.HealthCheckURL
	}
	if req.MaxFailures > 0 {
		h.cfg.ProxyPoolConfig.MaxFailures = req.MaxFailures
	}

	// Hot-reload pool
	if len(req.URLs) > 0 {
		poolCfg := proxypool.PoolConfig{
			URLs:                req.URLs,
			Strategy:            h.cfg.ProxyPoolConfig.Strategy,
			HealthCheckInterval: h.cfg.ProxyPoolConfig.HealthCheckInterval,
			HealthCheckURL:      h.cfg.ProxyPoolConfig.HealthCheckURL,
			MaxFailures:         h.cfg.ProxyPoolConfig.MaxFailures,
		}

		pool := proxypool.DefaultPool()
		if pool != nil {
			pool.Reload(req.URLs)
			log.WithField("count", len(req.URLs)).Info("[ProxyPool] Pool hot-reloaded via management API")
		} else {
			newPool := proxypool.InitPool(poolCfg)
			if newPool != nil {
				proxypool.StartHealthCheck(newPool, poolCfg)
			}
		}
	} else {
		// Empty URLs = disable pool
		if pool := proxypool.DefaultPool(); pool != nil {
			pool.Stop()
		}
		proxypool.InitPool(proxypool.PoolConfig{})
	}

	h.persist(c)
}
