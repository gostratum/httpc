package httpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gostratum/core/logx"
	"github.com/gostratum/httpc"
)

func TestRetriesOn503(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := httpc.New(
		httpc.WithBaseURL(server.URL),
		httpc.WithTimeout(2*time.Second),
		httpc.WithLogger(logx.NewNoopLogger()),
		httpc.WithRetry(true, 3), // Explicitly enable retry
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if status := resp.StatusCode(); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if atomic.LoadInt32(&attempts) < 2 {
		t.Fatalf("expected retry attempts, got %d", attempts)
	}
}
