package httpc

import (
	"encoding/json"
	"io"
	"net/http"
)

// Response wraps an http.Response with convenience helpers for decoding and
// inspecting metadata.
type Response struct {
	raw    *http.Response
	body   []byte
	loaded bool
	err    error
}

func newResponse(resp *http.Response) (*Response, error) {
	r := &Response{raw: resp}
	return r, nil
}

// ensureBody lazily reads the response body into memory so that helpers can
// operate even after the underlying http.Response body is closed.
func (r *Response) ensureBody() error {
	if r.loaded || r.err != nil {
		return r.err
	}
	defer func() {
		if r.raw != nil && r.raw.Body != nil {
			_ = r.raw.Body.Close()
		}
	}()

	if r.raw != nil && r.raw.Body != nil {
		b, err := io.ReadAll(r.raw.Body)
		if err != nil {
			r.err = err
			return err
		}
		r.body = b
	}
	r.loaded = true
	return nil
}

// StatusCode returns the HTTP status code.
func (r *Response) StatusCode() int {
	if r.raw == nil {
		return 0
	}
	return r.raw.StatusCode
}

// Header returns the first value for the given header key.
func (r *Response) Header(key string) string {
	if r.raw == nil {
		return ""
	}
	return r.raw.Header.Get(key)
}

// Headers exposes the response headers.
func (r *Response) Headers() http.Header {
	if r.raw == nil {
		return http.Header{}
	}
	return r.raw.Header.Clone()
}

// Bytes returns the response body as a byte slice.
func (r *Response) Bytes() ([]byte, error) {
	if err := r.ensureBody(); err != nil {
		return nil, err
	}
	return append([]byte(nil), r.body...), nil
}

// String returns the body as a string.
func (r *Response) String() (string, error) {
	b, err := r.Bytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeJSON decodes the response body into the supplied destination.
func (r *Response) DecodeJSON(dest any) error {
	if err := r.ensureBody(); err != nil {
		return err
	}
	if len(r.body) == 0 {
		return io.EOF
	}
	return json.Unmarshal(r.body, dest)
}

// IntoWriter copies the body into the provided writer.
func (r *Response) IntoWriter(w io.Writer) error {
	if err := r.ensureBody(); err != nil {
		return err
	}
	if len(r.body) == 0 {
		return nil
	}
	_, err := w.Write(r.body)
	return err
}

// Raw exposes the underlying http.Response for advanced consumers.
func (r *Response) Raw() *http.Response {
	return r.raw
}
