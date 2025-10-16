package breaker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// Manager manages host-scoped circuit breakers.
type Manager interface {
	Do(host string, fn func() (*http.Response, error)) (*http.Response, error)
}

// Config controls breaker behaviour.
type Config struct {
	Name          string
	Interval      time.Duration
	Timeout       time.Duration
	ReadyToTrip   func(counts gobreaker.Counts) bool
	OnStateChange func(name string, from gobreaker.State, to gobreaker.State)
}

// NewManager returns a default breaker manager keyed by host.
func NewManager(cfg Config) Manager {
	return &manager{
		config:   cfg,
		breakers: sync.Map{},
	}
}

type manager struct {
	config   Config
	breakers sync.Map
}

func (m *manager) Do(host string, fn func() (*http.Response, error)) (*http.Response, error) {
	if host == "" {
		return fn()
	}

	cb := m.get(host)
	result, err := cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	resp, _ := result.(*http.Response)
	return resp, nil
}

func (m *manager) get(host string) *gobreaker.CircuitBreaker {
	if cb, ok := m.breakers.Load(host); ok {
		return cb.(*gobreaker.CircuitBreaker)
	}

	settings := gobreaker.Settings{
		Name:        host,
		MaxRequests: 1,
		Interval:    m.config.Interval,
		Timeout:     defaultDuration(m.config.Timeout, 30*time.Second),
		ReadyToTrip: m.config.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if m.config.OnStateChange != nil {
				m.config.OnStateChange(name, from, to)
			}
		},
	}
	if settings.ReadyToTrip == nil {
		settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
			// Trip after 5 consecutive failures.
			return counts.ConsecutiveFailures >= 5
		}
	}

	cb := gobreaker.NewCircuitBreaker(settings)
	actual, _ := m.breakers.LoadOrStore(host, cb)
	return actual.(*gobreaker.CircuitBreaker)
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

type overrideKey struct{}

// WithOverride toggles breaker usage for the request.
func WithOverride(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, overrideKey{}, enabled)
}

// Enabled returns true if breaker should run for the request.
func Enabled(ctx context.Context, defaultEnabled bool) bool {
	if ctx == nil {
		return defaultEnabled
	}
	if v, ok := ctx.Value(overrideKey{}).(bool); ok {
		return v
	}
	return defaultEnabled
}

// NewMiddleware wraps a transport with breaker protection.
func NewMiddleware(m Manager) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if !Enabled(req.Context(), true) {
				return next.RoundTrip(req)
			}
			host := ""
			if req.URL != nil {
				host = req.URL.Host
			}
			return m.Do(host, func() (*http.Response, error) {
				return next.RoundTrip(req)
			})
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
