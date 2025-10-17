package httpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gostratum/core/logx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientHTTPMethods(t *testing.T) {
	t.Run("GET_request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"success"}`))
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("POST_request_with_JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":123}`))
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		body := map[string]interface{}{
			"name": "test",
		}
		resp, err := client.Post(context.Background(), "/users", body)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode())
	})

	t.Run("PUT_request_with_JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		body := map[string]interface{}{
			"name": "updated",
		}
		resp, err := client.Put(context.Background(), "/users/1", body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("PATCH_request_with_JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		body := map[string]interface{}{
			"status": "active",
		}
		resp, err := client.Patch(context.Background(), "/users/1", body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("DELETE_request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Delete(context.Background(), "/users/1")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode())
	})
}

func TestClientHeaders(t *testing.T) {
	t.Run("sets_custom_headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
			assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test",
			WithHeader("Authorization", "Bearer token123"),
			WithHeader("X-Custom-Header", "custom-value"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("sets_multiple_headers_via_map", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "value1", r.Header.Get("X-Header-1"))
			assert.Equal(t, "value2", r.Header.Get("X-Header-2"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test",
			WithHeaders(map[string]string{
				"X-Header-1": "value1",
				"X-Header-2": "value2",
			}),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("sets_user_agent", func(t *testing.T) {
		customUA := "MyApp/2.0"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, customUA, r.Header.Get("User-Agent"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithUserAgent(customUA),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestClientQueryParameters(t *testing.T) {
	t.Run("sets_single_query_parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test",
			WithQuery("param1", "value1"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("sets_multiple_query_parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))
			assert.Equal(t, "value2", r.URL.Query().Get("param2"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test",
			WithQueryMap(map[string]string{
				"param1": "value1",
				"param2": "value2",
			}),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestClientTimeout(t *testing.T) {
	t.Run("request_times_out", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithTimeout(50*time.Millisecond),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test")
		require.Error(t, err)
		// Accept either error message (depends on timing)
		assert.True(t,
			strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "Client.Timeout exceeded"),
			"expected timeout error, got: %v", err)
	})

	t.Run("per_request_timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithTimeout(1*time.Second), // High default timeout
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test",
			WithRequestTimeout(50*time.Millisecond), // Override with shorter timeout
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestClientContextCancellation(t *testing.T) {
	t.Run("respects_cancelled_context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = client.Get(ctx, "/test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("respects_context_deadline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err = client.Get(ctx, "/test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestClientBaseURL(t *testing.T) {
	t.Run("uses_base_url_for_relative_paths", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/users", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/api/users")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("handles_absolute_url", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		// Use absolute URL - should override base URL
		resp, err := client.Get(context.Background(), server.URL+"/test")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}

func TestClientContentType(t *testing.T) {
	t.Run("sets_json_content_type_automatically", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		body := map[string]string{"key": "value"}
		resp, err := client.Post(context.Background(), "/test", body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("overrides_content_type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		// Send JSON data but override content type
		body := map[string]string{"key": "value"}
		resp, err := client.Post(context.Background(), "/test", body,
			WithContentType("text/plain"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})
}
