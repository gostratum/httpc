package httpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gostratum/core/logx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_DecodeJSON(t *testing.T) {
	t.Run("unmarshal_valid_json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":   123,
				"name": "test",
			})
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		var result map[string]any
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)
		assert.Equal(t, float64(123), result["id"])
		assert.Equal(t, "test", result["name"])
	})

	t.Run("returns_error_for_invalid_json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json{"))
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		var result map[string]any
		err = resp.DecodeJSON(&result)
		assert.Error(t, err)
	})

	t.Run("handles_struct_unmarshaling", func(t *testing.T) {
		type User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(User{ID: 456, Name: "alice"})
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		var user User
		err = resp.DecodeJSON(&user)
		require.NoError(t, err)
		assert.Equal(t, 456, user.ID)
		assert.Equal(t, "alice", user.Name)
	})
}

func TestResponse_String(t *testing.T) {
	t.Run("returns_body_as_string", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hello world"))
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		str, err := resp.String()
		require.NoError(t, err)
		assert.Equal(t, "hello world", str)
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

		str, err := resp.String()
		require.NoError(t, err)
		assert.Equal(t, "", str)
	})
}

func TestResponse_Bytes(t *testing.T) {
	t.Run("returns_body_as_bytes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{0x01, 0x02, 0x03, 0x04})
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		bytes, err := resp.Bytes()
		require.NoError(t, err)
		assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, bytes)
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

		bytes, err := resp.Bytes()
		require.NoError(t, err)
		// Response with no body returns nil, not empty slice
		assert.Nil(t, bytes)
	})
}

func TestResponse_StatusCode(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{"returns_200", http.StatusOK, false},
		{"returns_201", http.StatusCreated, false},
		{"returns_204", http.StatusNoContent, false},
		{"returns_400", http.StatusBadRequest, false}, // By default, client doesn't error on 4xx/5xx
		{"returns_401", http.StatusUnauthorized, false},
		{"returns_403", http.StatusForbidden, false},
		{"returns_404", http.StatusNotFound, false},
		{"returns_500", http.StatusInternalServerError, false},
		{"returns_503", http.StatusServiceUnavailable, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client, err := New(
				WithBaseURL(server.URL),
				WithLogger(logx.NewNoopLogger()),
			)
			require.NoError(t, err)

			resp, err := client.Get(context.Background(), "/test")
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			require.NotNil(t, resp)
			assert.Equal(t, tc.statusCode, resp.StatusCode())
		})
	}
}

func TestResponse_Headers(t *testing.T) {
	t.Run("returns_response_headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom-Header", "custom-value")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		headers := resp.Headers()
		assert.Equal(t, "custom-value", headers.Get("X-Custom-Header"))
		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})
}

func TestResponse_IsSuccess(t *testing.T) {
	t.Run("returns_true_for_2xx", func(t *testing.T) {
		successCodes := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusAccepted,
			http.StatusNoContent,
		}

		for _, code := range successCodes {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))

			client, err := New(
				WithBaseURL(server.URL),
				WithLogger(logx.NewNoopLogger()),
			)
			require.NoError(t, err)

			resp, err := client.Get(context.Background(), "/test")
			require.NoError(t, err)
			isSuccess := resp.StatusCode() >= 200 && resp.StatusCode() < 300
			assert.True(t, isSuccess, "Expected %d to be success", code)

			server.Close()
		}
	})

	t.Run("returns_false_for_non_2xx", func(t *testing.T) {
		errorCodes := []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusInternalServerError,
			http.StatusServiceUnavailable,
		}

		for _, code := range errorCodes {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))

			client, err := New(
				WithBaseURL(server.URL),
				WithLogger(logx.NewNoopLogger()),
			)
			require.NoError(t, err)

			resp, _ := client.Get(context.Background(), "/test")
			require.NotNil(t, resp)
			isSuccess := resp.StatusCode() >= 200 && resp.StatusCode() < 300
			assert.False(t, isSuccess, "Expected %d to not be success", code)

			server.Close()
		}
	})
}

func TestResponse_Raw(t *testing.T) {
	t.Run("returns_underlying_http_response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "value")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test"))
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithLogger(logx.NewNoopLogger()),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test")
		require.NoError(t, err)

		rawResp := resp.Raw()
		require.NotNil(t, rawResp)
		assert.Equal(t, http.StatusOK, rawResp.StatusCode)
		assert.Equal(t, "value", rawResp.Header.Get("X-Test"))
	})
}
