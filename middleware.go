package httpc

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// Middleware allows chaining custom RoundTrippers around the base transport.
type Middleware func(http.RoundTripper) http.RoundTripper

func wrapTransport(base http.RoundTripper, middlewares ...Middleware) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	rt := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			rt = middlewares[i](rt)
		}
	}
	return rt
}

func newGzipMiddleware() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Accept-Encoding") == "" {
				req.Header.Set("Accept-Encoding", "gzip, deflate")
			}
			resp, err := next.RoundTrip(req)
			if err != nil || resp == nil || resp.Body == nil {
				return resp, err
			}

			switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
			case "gzip":
				reader, err := gzip.NewReader(resp.Body)
				if err != nil {
					_ = resp.Body.Close()
					return nil, err
				}
				resp.Body = wrapBody(reader, resp.Body)
				resp.Header.Del("Content-Encoding")
			case "deflate":
				reader := flate.NewReader(resp.Body)
				resp.Body = wrapBody(reader, resp.Body)
				resp.Header.Del("Content-Encoding")
			}

			return resp, nil
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func wrapBody(reader io.ReadCloser, original io.ReadCloser) io.ReadCloser {
	return &multiCloser{
		ReadCloser: reader,
		original:   original,
	}
}

type multiCloser struct {
	io.ReadCloser
	original io.ReadCloser
}

func (m *multiCloser) Close() error {
	err := m.ReadCloser.Close()
	_ = m.original.Close()
	return err
}
