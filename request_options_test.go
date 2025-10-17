package httpc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gostratum/core/logx"
	"github.com/gostratum/httpc/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithJSON(t *testing.T) {
	t.Run("sends_json_body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))

			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), `"name":"test"`)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		data := map[string]string{"name": "test"}
		resp, err := client.Post(context.Background(), "/test", data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestWithRaw(t *testing.T) {
	t.Run("sends_raw_body_with_content_type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))

			body, _ := io.ReadAll(r.Body)
			assert.Equal(t, "raw text data", string(body))

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Post(context.Background(), "/test", nil,
			WithRaw([]byte("raw text data"), "text/plain"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestWithForm(t *testing.T) {
	t.Run("sends_form_encoded_data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

			err := r.ParseForm()
			require.NoError(t, err)
			assert.Equal(t, "value1", r.Form.Get("key1"))
			assert.Equal(t, "value2", r.Form.Get("key2"))

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		formData := url.Values{}
		formData.Set("key1", "value1")
		formData.Set("key2", "value2")

		resp, err := client.Post(context.Background(), "/test", nil,
			WithForm(formData),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestWithAccept(t *testing.T) {
	t.Run("sets_accept_header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/xml", r.Header.Get("Accept"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test",
			WithAccept("application/xml"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestWithIdempotencyKey(t *testing.T) {
	t.Run("sets_idempotency_key_header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-key-123", r.Header.Get("Idempotency-Key"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Post(context.Background(), "/test", nil,
			WithIdempotencyKey("test-key-123"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("skips_empty_idempotency_key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Idempotency-Key"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Post(context.Background(), "/test", nil,
			WithIdempotencyKey(""),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestWithRequestAuth(t *testing.T) {
	t.Run("applies_request_level_auth", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		overrideAuth := auth.NewAPIKey(auth.APIKeyOptions{
			Key:  "test-api-key",
			In:   "header",
			Name: "X-API-Key",
		})
		resp, err := client.Get(context.Background(), "/test",
			WithRequestAuth(overrideAuth),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestResponse_IntoWriter(t *testing.T) {
	t.Run("writes_body_to_writer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test data for writer"))
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		var buf strings.Builder
		err = resp.IntoWriter(&buf)
		require.NoError(t, err)
		assert.Equal(t, "test data for writer", buf.String())
	})

	t.Run("handles_empty_body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		var buf strings.Builder
		err = resp.IntoWriter(&buf)
		require.NoError(t, err)
		assert.Equal(t, "", buf.String())
	})
}
