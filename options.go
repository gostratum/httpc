package httpc

import (
	"net/http"
	"time"

	"github.com/gostratum/core/logx"
	"github.com/gostratum/httpc/auth"
	"github.com/gostratum/httpc/breaker"
	"github.com/gostratum/httpc/retry"
)

// Option mutates the client configuration before a Client is constructed.
type Option func(*Config)

// WithConfig replaces the configuration struct. Typically used when loading
// configuration from configx/fx.
func WithConfig(cfg Config) Option {
	return func(c *Config) {
		*c = cfg
	}
}

// WithBaseURL sets the default base URL used when requests are built with a
// relative path.
func WithBaseURL(u string) Option {
	return func(c *Config) {
		c.BaseURL = u
	}
}

// WithTimeout overrides the default client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithRetry toggles retry behaviour at the client level.
func WithRetry(enabled bool, maxAttempts int) Option {
	return func(c *Config) {
		c.RetryEnabled = enabled
		if maxAttempts > 0 {
			c.RetryMaxAttempts = maxAttempts
		}
	}
}

// WithBreaker toggles the circuit breaker for outbound calls.
func WithBreaker(enabled bool) Option {
	return func(c *Config) {
		c.BreakerEnabled = enabled
	}
}

// WithTransport injects a custom transport for the underlying http.Client.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Config) {
		c.Transport = rt
	}
}

// WithHTTPClient injects an http.Client instance. When provided, the Timeout
// and Transport settings from the Config are applied on top.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// WithLogger sets the logger used for request logging middleware.
func WithLogger(l logx.Logger) Option {
	return func(c *Config) {
		c.Logger = l
	}
}

// WithAuth configures the default auth provider applied to every request (can
// be overridden per-request via ReqOption).
func WithAuth(p auth.AuthProvider) Option {
	return func(c *Config) {
		c.DefaultAuth = p
	}
}

// WithRetryPolicy replaces the default retry policy.
func WithRetryPolicy(p retry.Policy) Option {
	return func(c *Config) {
		c.RetryPolicy = p
	}
}

// WithBreakerManager injects a custom breaker manager implementation.
func WithBreakerManager(m breaker.Manager) Option {
	return func(c *Config) {
		c.Breaker = m
	}
}

// WithMiddleware appends a custom middleware to the transport chain.
func WithMiddleware(m Middleware) Option {
	return func(c *Config) {
		c.Middlewares = append(c.Middlewares, m)
	}
}

// WithUserAgent overrides the default User-Agent header applied to requests.
func WithUserAgent(ua string) Option {
	return func(c *Config) {
		c.UserAgent = ua
	}
}
