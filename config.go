package httpc

import (
	"net/http"
	"time"

	"github.com/gostratum/core/configx"
	"github.com/gostratum/core/logx"
	"github.com/gostratum/httpc/auth"
	"github.com/gostratum/httpc/breaker"
	"github.com/gostratum/httpc/retry"
)

// Config describes the runtime configuration for the HTTP client. It is
// intended to be populated via configx and then optionally overridden via
// functional options when constructing a client instance.
type Config struct {
	Env             string        `mapstructure:"env" default:"dev" validate:"oneof=dev prod"`
	BaseURL         string        `mapstructure:"base_url"`
	Timeout         time.Duration `mapstructure:"timeout" default:"10s"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" default:"100"`
	IdleConnTimeout time.Duration `mapstructure:"idle_conn_timeout" default:"90s"`

	RetryEnabled     bool          `mapstructure:"retry_enabled" default:"true"`
	RetryMaxAttempts int           `mapstructure:"retry_max_attempts" default:"3"`
	RetryBaseBackoff time.Duration `mapstructure:"retry_base_backoff" default:"200ms"`
	RetryMaxBackoff  time.Duration `mapstructure:"retry_max_backoff" default:"2s"`
	RetryOnStatuses  []int         `mapstructure:"retry_on_statuses" default:"502,503,504"`

	BreakerEnabled bool `mapstructure:"breaker_enabled" default:"false"`

	APIKey struct {
		Key  string `mapstructure:"key"`
		In   string `mapstructure:"in" default:"header"` // header|query
		Name string `mapstructure:"name" default:"X-API-Key"`
	} `mapstructure:"api_key"`

	Basic struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"basic"`

	JWT struct {
		Alg        string        `mapstructure:"alg" default:"RS256"`
		Issuer     string        `mapstructure:"issuer"`
		Audience   string        `mapstructure:"audience"`
		Kid        string        `mapstructure:"kid"`
		TTL        time.Duration `mapstructure:"ttl" default:"60s"`
		HMACSecret string        `mapstructure:"hmac_secret"`
		PrivatePEM string        `mapstructure:"private_pem"`
	} `mapstructure:"jwt"`

	// Runtime-only fields set via functional options (ignored by config loader).
	Transport   http.RoundTripper `mapstructure:"-"`
	Logger      logx.Logger       `mapstructure:"-"`
	DefaultAuth auth.AuthProvider `mapstructure:"-"`
	RetryPolicy retry.Policy      `mapstructure:"-"`
	Breaker     breaker.Manager   `mapstructure:"-"`
	Middlewares []Middleware      `mapstructure:"-"`
	HTTPClient  *http.Client      `mapstructure:"-"`
	UserAgent   string            `mapstructure:"-"`
}

// Prefix implements configx.Configurable.
func (Config) Prefix() string { return "httpc" }

// NewConfig loads the client configuration using the provided config loader.
func NewConfig(loader configx.Loader) (Config, error) {
	var cfg Config
	return cfg, loader.Bind(&cfg)
}

// applyDefaults ensures derived defaults that depend on other settings are set.
func (c *Config) applyDefaults() {
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 100
	}
	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = 90 * time.Second
	}
	if c.RetryMaxAttempts == 0 {
		c.RetryMaxAttempts = 3
	}
	if c.RetryBaseBackoff == 0 {
		c.RetryBaseBackoff = 200 * time.Millisecond
	}
	if c.RetryMaxBackoff == 0 {
		c.RetryMaxBackoff = 2 * time.Second
	}
	if len(c.RetryOnStatuses) == 0 {
		c.RetryOnStatuses = []int{502, 503, 504}
	}
	if c.JWT.Alg == "" {
		c.JWT.Alg = "RS256"
	}
	if c.JWT.TTL == 0 {
		c.JWT.TTL = time.Minute
	}
	if c.APIKey.Name == "" {
		c.APIKey.Name = "X-API-Key"
	}
	if c.APIKey.In == "" {
		c.APIKey.In = "header"
	}
}
