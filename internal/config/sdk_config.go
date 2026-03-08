// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

// SDKConfig represents the application's configuration, loaded from a YAML file.
type SDKConfig struct {
	// ProxyURL is the URL of an optional proxy server to use for outbound requests.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// ProxyURLs is a list of proxy URLs for multi-proxy pool rotation.
	// Takes precedence over ProxyURL when non-empty.
	ProxyURLs []string `yaml:"proxy-urls" json:"proxy-urls"`

	// ProxyPoolConfig configures the proxy pool behavior (strategy, health checking).
	ProxyPoolConfig ProxyPoolSDKConfig `yaml:"proxy-pool" json:"proxy-pool"`

	// ForceModelPrefix requires explicit model prefixes (e.g., "teamA/gemini-3-pro-preview")
	// to target prefixed credentials. When false, unprefixed model requests may use prefixed
	// credentials as well.
	ForceModelPrefix bool `yaml:"force-model-prefix" json:"force-model-prefix"`

	// RequestLog enables or disables detailed request logging functionality.
	RequestLog bool `yaml:"request-log" json:"request-log"`

	// APIKeys is a list of keys for authenticating clients to this proxy server.
	APIKeys []string `yaml:"api-keys" json:"api-keys"`

	// PassthroughHeaders controls whether upstream response headers are forwarded to downstream clients.
	// Default is false (disabled).
	PassthroughHeaders bool `yaml:"passthrough-headers" json:"passthrough-headers"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// NonStreamKeepAliveInterval controls how often blank lines are emitted for non-streaming responses.
	// <= 0 disables keep-alives. Value is in seconds.
	NonStreamKeepAliveInterval int `yaml:"nonstream-keepalive-interval,omitempty" json:"nonstream-keepalive-interval,omitempty"`
}

// StreamingConfig holds server streaming behavior configuration.
type StreamingConfig struct {
	// KeepAliveSeconds controls how often the server emits SSE heartbeats (": keep-alive\n\n").
	// <= 0 disables keep-alives. Default is 0.
	KeepAliveSeconds int `yaml:"keepalive-seconds,omitempty" json:"keepalive-seconds,omitempty"`

	// BootstrapRetries controls how many times the server may retry a streaming request before any bytes are sent,
	// to allow auth rotation / transient recovery.
	// <= 0 disables bootstrap retries. Default is 0.
	BootstrapRetries int `yaml:"bootstrap-retries,omitempty" json:"bootstrap-retries,omitempty"`
}

// ProxyPoolSDKConfig holds proxy pool behavior configuration.
type ProxyPoolSDKConfig struct {
	// Strategy determines the selection algorithm: "round-robin" (default), "random", "least-failed".
	Strategy string `yaml:"strategy,omitempty" json:"strategy,omitempty"`

	// HealthCheckInterval in seconds. 0 disables health checking. Default: 60.
	HealthCheckInterval int `yaml:"health-check-interval,omitempty" json:"health-check-interval,omitempty"`

	// HealthCheckURL is the URL to probe for health checks. Default: "https://chatgpt.com/favicon.ico".
	HealthCheckURL string `yaml:"health-check-url,omitempty" json:"health-check-url,omitempty"`

	// MaxFailures is the consecutive failure count before marking unhealthy. Default: 3.
	MaxFailures int `yaml:"max-failures,omitempty" json:"max-failures,omitempty"`
}

