package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gostratum/core/logx"
)

// Policy determines if and when a request should be retried.
type Policy interface {
	ShouldRetry(req *http.Request, resp *http.Response, err error, attempt int, force bool) (time.Duration, bool)
}

// PolicyConfig configures the default retry strategy.
type PolicyConfig struct {
	MaxAttempts    int
	BaseBackoff    time.Duration
	MaxBackoff     time.Duration
	StatusCodes    []int
	Env            string
	IdempotentOnly bool
}

// NewPolicy constructs a Policy using exponential backoff with jitter.
func NewPolicy(cfg PolicyConfig) Policy {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 200 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 2 * time.Second
	}

	codeSet := make(map[int]struct{}, len(cfg.StatusCodes))
	for _, code := range cfg.StatusCodes {
		codeSet[code] = struct{}{}
	}

	return &policy{
		maxAttempts:    cfg.MaxAttempts,
		baseBackoff:    cfg.BaseBackoff,
		maxBackoff:     cfg.MaxBackoff,
		statusCodes:    codeSet,
		idempotentOnly: cfg.IdempotentOnly,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type policy struct {
	maxAttempts    int
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	statusCodes    map[int]struct{}
	idempotentOnly bool

	mu   sync.Mutex
	rand *rand.Rand
}

func (p *policy) ShouldRetry(req *http.Request, resp *http.Response, err error, attempt int, force bool) (time.Duration, bool) {
	if attempt >= p.maxAttempts {
		return 0, false
	}

	if !force && p.idempotentOnly && !isIdempotent(req.Method) {
		return 0, false
	}

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, false
		}
		if isNetErrorRetryable(err) {
			return p.backoff(attempt), true
		}
		return 0, false
	}

	if resp == nil {
		return 0, false
	}

	if _, ok := p.statusCodes[resp.StatusCode]; ok {
		return p.backoff(attempt), true
	}

	return 0, false
}

func (p *policy) backoff(attempt int) time.Duration {
	factor := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(p.baseBackoff) * factor)
	if delay > p.maxBackoff {
		delay = p.maxBackoff
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	jitter := p.rand.Float64() * float64(delay) * 0.2
	return delay + time.Duration(jitter)
}

func isIdempotent(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isNetErrorRetryable(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

type policyKey struct{}
type forceKey struct{}

// WithPolicy stores the policy in the request context.
func WithPolicy(ctx context.Context, p Policy) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, policyKey{}, p)
}

// PolicyFromContext retrieves a policy from context.
func PolicyFromContext(ctx context.Context) Policy {
	if ctx == nil {
		return nil
	}
	p, _ := ctx.Value(policyKey{}).(Policy)
	return p
}

// WithForce marks a request as retryable regardless of method idempotency.
func WithForce(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, forceKey{}, true)
}

// IsForce returns true if the context requested forced retries.
func IsForce(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	force, _ := ctx.Value(forceKey{}).(bool)
	return force
}

// NewMiddleware constructs a retry middleware.
func NewMiddleware(defaultPolicy Policy, logger logx.Logger) func(http.RoundTripper) http.RoundTripper {
	if logger == nil {
		logger = logx.NewNoopLogger()
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			policy := PolicyFromContext(req.Context())
			if policy == nil {
				policy = defaultPolicy
			}
			if policy == nil {
				return next.RoundTrip(req)
			}

			force := IsForce(req.Context())
			attempt := 1

			orig := req

			for {
				currentReq, err := prepareAttemptRequest(orig, attempt)
				if err != nil {
					return nil, err
				}

				resp, err := next.RoundTrip(currentReq)

				delay, retryable := policy.ShouldRetry(currentReq, resp, err, attempt, force)
				if !retryable {
					return resp, err
				}

				if resp != nil && resp.Body != nil {
					_ = resp.Body.Close()
				}

				attempt++
				if err := waitWithContext(req.Context(), delay); err != nil {
					return nil, fmt.Errorf("retry interrupted: %w", err)
				}
				logger.Debug("retrying http request",
					logx.String("method", req.Method),
					logx.String("url", req.URL.String()),
					logx.Int("attempt", attempt),
				)
			}
		})
	}
}

func waitWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func prepareAttemptRequest(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 1 {
		return req, nil
	}

	clone := req.Clone(req.Context())

	// Handle requests with no body (like GET)
	if req.GetBody == nil {
		// For GET and other methods without a body, we can retry without issue
		return clone, nil
	}

	// For requests with a body, we need to get a fresh copy
	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("reset body: %w", err)
	}
	clone.Body = body
	clone.GetBody = req.GetBody
	return clone, nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
