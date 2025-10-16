package httpc_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/gostratum/httpc"
	"github.com/gostratum/httpc/auth"
)

type captureTransport struct {
	req  *http.Request
	body []byte
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	if req.Body != nil {
		defer req.Body.Close()
		c.body, _ = io.ReadAll(req.Body)
	}
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
}

func TestClientJSONRequest(t *testing.T) {
	capture := &captureTransport{}
	client, err := httpc.New(
		httpc.WithTransport(capture),
		httpc.WithRetry(false, 0),
		httpc.WithBaseURL("https://api.example.com"),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Post(context.Background(), "/items", map[string]any{"name": "demo"})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if capture.req == nil {
		t.Fatalf("request not captured")
	}
	if ct := capture.req.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", ct)
	}
	if !strings.Contains(string(capture.body), `"name":"demo"`) {
		t.Fatalf("unexpected body: %s", capture.body)
	}

	var decoded map[string]bool
	if err := resp.DecodeJSON(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestClientMultipart(t *testing.T) {
	capture := &captureTransport{}
	client, err := httpc.New(
		httpc.WithTransport(capture),
		httpc.WithRetry(false, 0),
		httpc.WithAuth(auth.NewAPIKey(auth.APIKeyOptions{Key: "sekret"})),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	fileContent := "hello world"
	err = func() error {
		_, err := client.Post(context.Background(), "https://upload.example.com/upload",
			httpc.WithMultipart([]httpc.MultipartFile{
				{
					FieldName: "file",
					FileName:  "hello.txt",
					Reader:    strings.NewReader(fileContent),
				},
			}, map[string]string{"title": "greeting"}),
		)
		return err
	}()
	if err != nil {
		t.Fatalf("post multipart: %v", err)
	}

	if capture.req == nil {
		t.Fatalf("request not captured")
	}
	if !strings.HasPrefix(capture.req.Header.Get("Content-Type"), "multipart/form-data") {
		t.Fatalf("expected multipart content-type, got %q", capture.req.Header.Get("Content-Type"))
	}

	if capture.req.Header.Get("X-API-Key") != "sekret" {
		t.Fatalf("expected API key header applied")
	}

	if !bytes.Contains(capture.body, []byte(fileContent)) {
		t.Fatalf("multipart body missing file content: %s", capture.body)
	}
	if !bytes.Contains(capture.body, []byte(`name="title"`)) {
		t.Fatalf("multipart body missing field: %s", capture.body)
	}
}
