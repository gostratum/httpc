package httpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gostratum/core/logx"
	"github.com/gostratum/httpc/auth"
	"github.com/gostratum/httpc/breaker"
	"github.com/gostratum/httpc/retry"
)

// Client represents the public contract for the HTTP client.
type Client interface {
	Do(ctx context.Context, req *Request) (*Response, error)
	Get(ctx context.Context, url string, opts ...ReqOption) (*Response, error)
	Post(ctx context.Context, url string, body any, opts ...ReqOption) (*Response, error)
	Put(ctx context.Context, url string, body any, opts ...ReqOption) (*Response, error)
	Patch(ctx context.Context, url string, body any, opts ...ReqOption) (*Response, error)
	Delete(ctx context.Context, url string, opts ...ReqOption) (*Response, error)
}

const defaultUserAgent = "httpc/0"

type client struct {
	cfg         Config
	httpClient  *http.Client
	retryPolicy retry.Policy
	breakerMgr  breaker.Manager
}

// New constructs a Client with the supplied options applied.
func New(opts ...Option) (Client, error) {
	cfg := Config{}
	cfg.applyDefaults()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.applyDefaults()

	if cfg.DefaultAuth == nil {
		switch {
		case cfg.APIKey.Key != "":
			cfg.DefaultAuth = auth.NewAPIKey(auth.APIKeyOptions{
				Key:  cfg.APIKey.Key,
				In:   cfg.APIKey.In,
				Name: cfg.APIKey.Name,
			})
		case cfg.Basic.Username != "":
			cfg.DefaultAuth = auth.NewBasic(auth.BasicOptions{
				Username: cfg.Basic.Username,
				Password: cfg.Basic.Password,
			})
		case cfg.JWT.Alg != "" && (cfg.JWT.HMACSecret != "" || cfg.JWT.PrivatePEM != ""):
			jwtOpts := auth.JWTOptions{
				Alg:      cfg.JWT.Alg,
				Issuer:   cfg.JWT.Issuer,
				Audience: cfg.JWT.Audience,
				Kid:      cfg.JWT.Kid,
				TTL:      cfg.JWT.TTL,
			}
			switch strings.ToUpper(cfg.JWT.Alg) {
			case "HS256":
				secret, err := auth.LoadHMACSecret(cfg.JWT.HMACSecret)
				if err != nil {
					return nil, fmt.Errorf("load jwt hmac secret: %w", err)
				}
				jwtOpts.HMACSecret = secret
			case "RS256":
				if cfg.JWT.PrivatePEM == "" {
					return nil, errors.New("jwt private_pem is required for rs256")
				}
				key, err := auth.LoadRSAPrivateKey(cfg.JWT.PrivatePEM)
				if err != nil {
					return nil, fmt.Errorf("load jwt private key: %w", err)
				}
				jwtOpts.PrivateKey = key
			default:
				return nil, fmt.Errorf("unsupported jwt alg %q", cfg.JWT.Alg)
			}
			jwtProvider, err := auth.NewJWT(jwtOpts)
			if err != nil {
				return nil, err
			}
			cfg.DefaultAuth = jwtProvider
		}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logx.NewNoopLogger()
	}

	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}

	baseTransport := cfg.Transport
	if baseTransport == nil {
		baseTransport = defaultTransport(cfg)
	}

	retryPolicy := cfg.RetryPolicy
	if retryPolicy == nil && cfg.RetryEnabled {
		retryPolicy = retry.NewPolicy(retry.PolicyConfig{
			MaxAttempts:    cfg.RetryMaxAttempts,
			BaseBackoff:    cfg.RetryBaseBackoff,
			MaxBackoff:     cfg.RetryMaxBackoff,
			StatusCodes:    cfg.RetryOnStatuses,
			Env:            cfg.Env,
			IdempotentOnly: true,
		})
	}

	breakerMgr := cfg.Breaker
	if breakerMgr == nil && cfg.BreakerEnabled {
		breakerMgr = breaker.NewManager(breaker.Config{})
	}

	transport := wrapTransport(baseTransport,
		newGzipMiddleware(),
	)

	if cfg.BreakerEnabled {
		if breakerMgr == nil {
			return nil, errors.New("breaker enabled but no breaker manager configured")
		}
		transport = wrapTransport(transport, breaker.NewMiddleware(breakerMgr))
	}

	if cfg.RetryEnabled {
		if retryPolicy == nil {
			return nil, errors.New("retry enabled but no policy configured")
		}
		transport = wrapTransport(transport, retry.NewMiddleware(retryPolicy, logger))
	}

	for _, mw := range cfg.Middlewares {
		if mw != nil {
			transport = wrapTransport(transport, mw)
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		}
	} else {
		httpClient.Timeout = cfg.Timeout
		httpClient.Transport = transport
	}

	return &client{
		cfg:         cfg,
		httpClient:  httpClient,
		retryPolicy: retryPolicy,
		breakerMgr:  breakerMgr,
	}, nil
}

// Do executes the supplied Request.
func (c *client) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r := req.clone()

	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	httpReq, err := r.buildHTTPRequest(ctx, c.cfg)
	if err != nil {
		return nil, err
	}

	authProvider := r.authProvider
	if authProvider == nil {
		authProvider = c.cfg.DefaultAuth
	}
	if authProvider != nil {
		if err := authProvider.Apply(httpReq); err != nil {
			return nil, fmt.Errorf("apply auth: %w", err)
		}
	}

	policy := r.retryPolicy
	if policy == nil {
		policy = c.retryPolicy
	}
	if policy != nil {
		ctx = retry.WithPolicy(ctx, policy)
		if r.forceRetry {
			ctx = retry.WithForce(ctx)
		}
	}

	if r.breakerToggle != nil {
		ctx = breaker.WithOverride(ctx, *r.breakerToggle)
	}

	httpReq = httpReq.WithContext(ctx)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	return newResponse(resp)
}

func (c *client) Get(ctx context.Context, url string, opts ...ReqOption) (*Response, error) {
	return c.execute(ctx, "GET", url, nil, opts...)
}

func (c *client) Post(ctx context.Context, url string, body any, opts ...ReqOption) (*Response, error) {
	return c.execute(ctx, "POST", url, body, opts...)
}

func (c *client) Put(ctx context.Context, url string, body any, opts ...ReqOption) (*Response, error) {
	return c.execute(ctx, "PUT", url, body, opts...)
}

func (c *client) Patch(ctx context.Context, url string, body any, opts ...ReqOption) (*Response, error) {
	return c.execute(ctx, "PATCH", url, body, opts...)
}

func (c *client) Delete(ctx context.Context, url string, opts ...ReqOption) (*Response, error) {
	return c.execute(ctx, "DELETE", url, nil, opts...)
}

func (c *client) execute(ctx context.Context, method, target string, body any, opts ...ReqOption) (*Response, error) {
	opts = append(normalizeBody(body), opts...)
	req := newRequest(method, target, opts...)
	return c.Do(ctx, req)
}

func normalizeBody(body any) []ReqOption {
	if body == nil {
		return nil
	}
	switch v := body.(type) {
	case ReqOption:
		return []ReqOption{v}
	case []byte:
		return []ReqOption{WithRaw(v, "application/octet-stream")}
	case string:
		return []ReqOption{WithRaw([]byte(v), "text/plain; charset=utf-8")}
	case io.ReadSeeker:
		return []ReqOption{withReadSeeker(v)}
	case io.Reader:
		data, err := io.ReadAll(v)
		if err != nil {
			return []ReqOption{func(r *Request) {
				r.bodyFactory = func() (io.ReadCloser, int64, string, error) { return nil, 0, "", err }
			}}
		}
		return []ReqOption{WithRaw(data, "application/octet-stream")}
	default:
		return []ReqOption{WithJSON(v)}
	}
}

func withReadSeeker(rs io.ReadSeeker) ReqOption {
	return func(r *Request) {
		r.bodyFactory = func() (io.ReadCloser, int64, string, error) {
			if _, err := rs.Seek(0, io.SeekStart); err != nil {
				return nil, 0, "", err
			}
			data, err := io.ReadAll(rs)
			if err != nil {
				return nil, 0, "", err
			}
			return io.NopCloser(bytes.NewReader(data)), int64(len(data)), "application/octet-stream", nil
		}
	}
}

func defaultTransport(cfg Config) http.RoundTripper {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          cfg.MaxIdleConns,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}
