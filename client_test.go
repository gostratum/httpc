package httpc

import (
	"net/http"
	"testing"
	"time"

	"github.com/gostratum/core/logx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates_client_with_defaults", func(t *testing.T) {
		client, err := New()
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_base_url", func(t *testing.T) {
		baseURL := "https://api.example.com"
		client, err := New(WithBaseURL(baseURL))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_timeout", func(t *testing.T) {
		timeout := 5 * time.Second
		client, err := New(WithTimeout(timeout))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_logger", func(t *testing.T) {
		logger := logx.NewNoopLogger()
		client, err := New(WithLogger(logger))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_retry", func(t *testing.T) {
		client, err := New(WithRetry(true, 5))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_breaker", func(t *testing.T) {
		client, err := New(WithBreaker(true))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_user_agent", func(t *testing.T) {
		userAgent := "MyApp/1.0"
		client, err := New(WithUserAgent(userAgent))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_custom_transport", func(t *testing.T) {
		transport := &http.Transport{
			MaxIdleConns: 50,
		}
		client, err := New(WithTransport(transport))
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("creates_client_with_multiple_options", func(t *testing.T) {
		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithTimeout(10*time.Second),
			WithLogger(logx.NewNoopLogger()),
			WithRetry(true, 3),
			WithUserAgent("MyApp/1.0"),
		)
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}

func TestConfig_applyDefaults(t *testing.T) {
	t.Run("applies_default_timeout", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, 10*time.Second, cfg.Timeout)
	})

	t.Run("applies_default_max_idle_conns", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, 100, cfg.MaxIdleConns)
	})

	t.Run("applies_default_idle_conn_timeout", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, 90*time.Second, cfg.IdleConnTimeout)
	})

	t.Run("applies_default_retry_max_attempts", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, 3, cfg.RetryMaxAttempts)
	})

	t.Run("applies_default_retry_base_backoff", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, 200*time.Millisecond, cfg.RetryBaseBackoff)
	})

	t.Run("applies_default_retry_max_backoff", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, 2*time.Second, cfg.RetryMaxBackoff)
	})

	t.Run("applies_default_retry_on_statuses", func(t *testing.T) {
		cfg := Config{}
		cfg.applyDefaults()
		assert.Equal(t, []int{502, 503, 504}, cfg.RetryOnStatuses)
	})

	t.Run("does_not_override_existing_timeout", func(t *testing.T) {
		cfg := Config{Timeout: 5 * time.Second}
		cfg.applyDefaults()
		assert.Equal(t, 5*time.Second, cfg.Timeout)
	})

	t.Run("does_not_override_existing_retry_statuses", func(t *testing.T) {
		cfg := Config{RetryOnStatuses: []int{500, 502}}
		cfg.applyDefaults()
		assert.Equal(t, []int{500, 502}, cfg.RetryOnStatuses)
	})
}

func TestConfigPrefix(t *testing.T) {
	t.Run("returns_correct_prefix", func(t *testing.T) {
		cfg := Config{}
		assert.Equal(t, "httpc", cfg.Prefix())
	})
}

func TestWithConfig(t *testing.T) {
	t.Run("replaces_entire_config", func(t *testing.T) {
		originalCfg := Config{
			BaseURL: "https://api.example.com",
			Timeout: 5 * time.Second,
		}

		cfg := Config{}
		opt := WithConfig(originalCfg)
		opt(&cfg)

		assert.Equal(t, originalCfg.BaseURL, cfg.BaseURL)
		assert.Equal(t, originalCfg.Timeout, cfg.Timeout)
	})
}

func TestWithBaseURL(t *testing.T) {
	t.Run("sets_base_url", func(t *testing.T) {
		cfg := Config{}
		opt := WithBaseURL("https://api.example.com")
		opt(&cfg)
		assert.Equal(t, "https://api.example.com", cfg.BaseURL)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("sets_timeout", func(t *testing.T) {
		cfg := Config{}
		opt := WithTimeout(15 * time.Second)
		opt(&cfg)
		assert.Equal(t, 15*time.Second, cfg.Timeout)
	})
}

func TestWithRetry(t *testing.T) {
	t.Run("enables_retry", func(t *testing.T) {
		cfg := Config{}
		opt := WithRetry(true, 5)
		opt(&cfg)
		assert.True(t, cfg.RetryEnabled)
		assert.Equal(t, 5, cfg.RetryMaxAttempts)
	})

	t.Run("disables_retry", func(t *testing.T) {
		cfg := Config{RetryEnabled: true}
		opt := WithRetry(false, 3)
		opt(&cfg)
		assert.False(t, cfg.RetryEnabled)
	})

	t.Run("ignores_zero_max_attempts", func(t *testing.T) {
		cfg := Config{RetryMaxAttempts: 3}
		opt := WithRetry(true, 0)
		opt(&cfg)
		assert.True(t, cfg.RetryEnabled)
		assert.Equal(t, 3, cfg.RetryMaxAttempts)
	})
}

func TestWithBreaker(t *testing.T) {
	t.Run("enables_breaker", func(t *testing.T) {
		cfg := Config{}
		opt := WithBreaker(true)
		opt(&cfg)
		assert.True(t, cfg.BreakerEnabled)
	})

	t.Run("disables_breaker", func(t *testing.T) {
		cfg := Config{BreakerEnabled: true}
		opt := WithBreaker(false)
		opt(&cfg)
		assert.False(t, cfg.BreakerEnabled)
	})
}

func TestWithLogger(t *testing.T) {
	t.Run("sets_logger", func(t *testing.T) {
		logger := logx.NewNoopLogger()
		cfg := Config{}
		opt := WithLogger(logger)
		opt(&cfg)
		assert.Equal(t, logger, cfg.Logger)
	})
}

func TestWithUserAgent(t *testing.T) {
	t.Run("sets_user_agent", func(t *testing.T) {
		cfg := Config{}
		opt := WithUserAgent("CustomAgent/2.0")
		opt(&cfg)
		assert.Equal(t, "CustomAgent/2.0", cfg.UserAgent)
	})
}

func TestWithTransport(t *testing.T) {
	t.Run("sets_transport", func(t *testing.T) {
		transport := &http.Transport{MaxIdleConns: 50}
		cfg := Config{}
		opt := WithTransport(transport)
		opt(&cfg)
		assert.Equal(t, transport, cfg.Transport)
	})
}

func TestWithHTTPClient(t *testing.T) {
	t.Run("sets_http_client", func(t *testing.T) {
		httpClient := &http.Client{Timeout: 30 * time.Second}
		cfg := Config{}
		opt := WithHTTPClient(httpClient)
		opt(&cfg)
		assert.Equal(t, httpClient, cfg.HTTPClient)
	})
}
