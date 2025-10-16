package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gostratum/httpc/auth"
	"github.com/gostratum/httpc/retry"
)

// ReqOption applies configuration to a Request before it is executed.
type ReqOption func(*Request)

// MultipartFile describes a file part within a multipart/form-data request.
type MultipartFile struct {
	FieldName   string
	FileName    string
	Reader      io.Reader
	ContentType string
}

type bodyProvider func() (io.ReadCloser, int64, string, error)

// Request captures the data required to execute an HTTP call.
type Request struct {
	method string
	url    string

	headers http.Header
	queries url.Values

	timeout       time.Duration
	authProvider  auth.AuthProvider
	retryPolicy   retry.Policy
	forceRetry    bool
	breakerToggle *bool

	bodyFactory bodyProvider
	contentType string
	accept      string
}

// newRequest constructs a Request with defaults and applies the provided
// options. Users typically rely on the helper methods on Client instead.
func newRequest(method, target string, opts ...ReqOption) *Request {
	req := &Request{
		method:  strings.ToUpper(method),
		url:     target,
		headers: make(http.Header),
		queries: make(url.Values),
	}
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// Method returns the HTTP method associated with the request.
func (r *Request) Method() string { return r.method }

// URL returns the raw URL (possibly relative) associated with the request.
func (r *Request) URL() string { return r.url }

// clone produces a deep copy used when attempts are retried.
func (r *Request) clone() *Request {
	clone := &Request{
		method:        r.method,
		url:           r.url,
		timeout:       r.timeout,
		authProvider:  r.authProvider,
		retryPolicy:   r.retryPolicy,
		forceRetry:    r.forceRetry,
		breakerToggle: r.breakerToggle,
		contentType:   r.contentType,
		accept:        r.accept,
		bodyFactory:   r.bodyFactory,
		headers:       make(http.Header, len(r.headers)),
		queries:       make(url.Values, len(r.queries)),
	}
	for k, vv := range r.headers {
		cp := make([]string, len(vv))
		copy(cp, vv)
		clone.headers[k] = cp
	}
	for k, vv := range r.queries {
		cp := make([]string, len(vv))
		copy(cp, vv)
		clone.queries[k] = cp
	}
	return clone
}

// buildHTTPRequest expands the request into a concrete *http.Request using the
// supplied base URL and context.
func (r *Request) buildHTTPRequest(ctx context.Context, cfg Config) (*http.Request, error) {
	baseURL := cfg.BaseURL
	target := r.url
	if baseURL != "" && !isAbsoluteURL(target) {
		target = strings.TrimRight(baseURL, "/")
		if !strings.HasPrefix(r.url, "/") {
			target += "/"
		}
		target = target + strings.TrimLeft(r.url, "/")
	}

	if len(r.queries) > 0 {
		u, err := url.Parse(target)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for k, vv := range r.queries {
			for _, v := range vv {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
		target = u.String()
	}

	var body io.ReadCloser
	var contentLength int64
	if r.bodyFactory != nil {
		rc, cl, ctype, err := r.bodyFactory()
		if err != nil {
			return nil, err
		}
		body = rc
		contentLength = cl
		if ctype != "" {
			r.contentType = ctype
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, r.method, target, body)
	if err != nil {
		if body != nil {
			_ = body.Close()
		}
		return nil, err
	}

	if r.bodyFactory != nil {
		httpReq.GetBody = func() (io.ReadCloser, error) {
			rc, _, _, err := r.bodyFactory()
			return rc, err
		}
	}

	// Copy headers
	for k, vv := range r.headers {
		for _, v := range vv {
			httpReq.Header.Add(k, v)
		}
	}

	if cfg.UserAgent != "" && httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", cfg.UserAgent)
	}
	if r.contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", r.contentType)
	}
	if r.accept != "" && httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", r.accept)
	}
	if contentLength >= 0 {
		httpReq.ContentLength = contentLength
	}

	return httpReq, nil
}

// WithHeader sets a header value on the outgoing request.
func WithHeader(key, value string) ReqOption {
	return func(r *Request) {
		r.headers.Set(key, value)
	}
}

// WithHeaders sets multiple headers. Existing values for the same key are
// replaced.
func WithHeaders(headers map[string]string) ReqOption {
	return func(r *Request) {
		for k, v := range headers {
			r.headers.Set(k, v)
		}
	}
}

// WithAccept sets the Accept header.
func WithAccept(value string) ReqOption {
	return func(r *Request) {
		r.accept = value
	}
}

// WithContentType sets the Content-Type header for the body.
func WithContentType(value string) ReqOption {
	return func(r *Request) {
		r.contentType = value
	}
}

// WithIdempotencyKey sets the Idempotency-Key header.
func WithIdempotencyKey(key string) ReqOption {
	return func(r *Request) {
		if key != "" {
			r.headers.Set("Idempotency-Key", key)
		}
	}
}

// WithQuery appends a query parameter to the request.
func WithQuery(key, value string) ReqOption {
	return func(r *Request) {
		r.queries.Add(key, value)
	}
}

// WithQueryMap appends multiple query parameters.
func WithQueryMap(values map[string]string) ReqOption {
	return func(r *Request) {
		for k, v := range values {
			r.queries.Add(k, v)
		}
	}
}

// WithRequestTimeout overrides the timeout for this specific request.
func WithRequestTimeout(d time.Duration) ReqOption {
	return func(r *Request) {
		r.timeout = d
	}
}

// WithRequestAuth overrides the auth provider used for the request.
func WithRequestAuth(provider auth.AuthProvider) ReqOption {
	return func(r *Request) {
		r.authProvider = provider
	}
}

// WithRequestRetry overrides the retry policy for this request.
func WithRequestRetry(policy retry.Policy) ReqOption {
	return func(r *Request) {
		r.retryPolicy = policy
	}
}

// WithRequestRetryForce marks the request as retryable even for non-idempotent
// methods.
func WithRequestRetryForce() ReqOption {
	return func(r *Request) {
		r.forceRetry = true
	}
}

// WithRequestBreaker toggles the circuit breaker for this request.
func WithRequestBreaker(enabled bool) ReqOption {
	return func(r *Request) {
		r.breakerToggle = ptr(enabled)
	}
}

// WithRaw sets an arbitrary payload with a custom Content-Type.
func WithRaw(body []byte, contentType string) ReqOption {
	return func(r *Request) {
		r.bodyFactory = func() (io.ReadCloser, int64, string, error) {
			buf := make([]byte, len(body))
			copy(buf, body)
			return io.NopCloser(bytes.NewReader(buf)), int64(len(buf)), contentType, nil
		}
	}
}

// WithJSON serialises the provided value as JSON and applies the appropriate
// Content-Type.
func WithJSON(v any) ReqOption {
	return func(r *Request) {
		r.bodyFactory = func() (io.ReadCloser, int64, string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, 0, "", err
			}
			return io.NopCloser(bytes.NewReader(b)), int64(len(b)), "application/json", nil
		}
		r.accept = choose(r.accept, "application/json")
	}
}

// WithForm encodes the provided values as application/x-www-form-urlencoded.
func WithForm(values url.Values) ReqOption {
	return func(r *Request) {
		r.bodyFactory = func() (io.ReadCloser, int64, string, error) {
			encoded := values.Encode()
			return io.NopCloser(strings.NewReader(encoded)), int64(len(encoded)), "application/x-www-form-urlencoded", nil
		}
	}
}

// WithMultipart builds a multipart/form-data body.
func WithMultipart(files []MultipartFile, fields map[string]string) ReqOption {
	return func(r *Request) {
		r.bodyFactory = func() (io.ReadCloser, int64, string, error) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			for k, v := range fields {
				if err := writer.WriteField(k, v); err != nil {
					return nil, 0, "", fmt.Errorf("write field %q: %w", k, err)
				}
			}

			for _, file := range files {
				var part io.Writer
				var err error
				if file.ContentType != "" {
					hdr := make(textproto.MIMEHeader)
					hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, file.FieldName, path.Base(file.FileName)))
					hdr.Set("Content-Type", file.ContentType)
					part, err = writer.CreatePart(hdr)
				} else {
					part, err = writer.CreateFormFile(file.FieldName, path.Base(file.FileName))
				}
				if err != nil {
					return nil, 0, "", fmt.Errorf("create part for %q: %w", file.FieldName, err)
				}
				if _, err := io.Copy(part, file.Reader); err != nil {
					return nil, 0, "", fmt.Errorf("copy part %q: %w", file.FieldName, err)
				}
			}

			if err := writer.Close(); err != nil {
				return nil, 0, "", fmt.Errorf("close multipart writer: %w", err)
			}

			body := buf.Bytes()
			return io.NopCloser(bytes.NewReader(body)), int64(len(body)), writer.FormDataContentType(), nil
		}
		// Accept header is typically omitted for multipart.
	}
}

func choose(current, fallback string) string {
	if current != "" {
		return current
	}
	return fallback
}

func isAbsoluteURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme != ""
}

func ptr[T any](v T) *T {
	return &v
}
