package httpc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gostratum/httpc/auth"
)

func TestAPIKeyHeader(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	provider := auth.NewAPIKey(auth.APIKeyOptions{Key: "sekret", In: "header", Name: "X-API-Key"})
	if err := provider.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := req.Header.Get("X-API-Key"); got != "sekret" {
		t.Fatalf("expected header applied, got %q", got)
	}
}

func TestAPIKeyQuery(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	provider := auth.NewAPIKey(auth.APIKeyOptions{Key: "sekret", In: "query", Name: "api_key"})
	if err := provider.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := req.URL.Query().Get("api_key"); got != "sekret" {
		t.Fatalf("expected query applied, got %q", got)
	}
}

func TestBasicAuth(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	provider := auth.NewBasic(auth.BasicOptions{Username: "user", Password: "pass"})
	if err := provider.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "Basic ") {
		t.Fatalf("expected basic authorization header, got %q", got)
	}
}

func TestJWTAuthHS256(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	provider, err := auth.NewJWT(auth.JWTOptions{
		Alg:        "HS256",
		Issuer:     "me",
		Audience:   "you",
		TTL:        time.Minute,
		HMACSecret: []byte("super-secret"),
	})
	if err != nil {
		t.Fatalf("new jwt: %v", err)
	}
	if err := provider.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}

	raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if raw == "" {
		t.Fatalf("expected bearer token")
	}

	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			t.Fatalf("unexpected alg %s", t.Method.Alg())
		}
		return []byte("super-secret"), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("token invalid: %v", err)
	}
}

func TestJWTAuthRS256(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	provider, err := auth.NewJWT(auth.JWTOptions{
		Alg:        "RS256",
		Issuer:     "issuer",
		Audience:   "audience",
		TTL:        time.Minute,
		PrivateKey: key,
	}, auth.WithClaims(func(claims jwt.MapClaims) {
		claims["role"] = "admin"
	}))
	if err != nil {
		t.Fatalf("new jwt: %v", err)
	}
	if err := provider.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if raw == "" {
		t.Fatalf("expected bearer token")
	}

	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("token invalid: %v", err)
	}

	claims, _ := token.Claims.(jwt.MapClaims)
	if claims["iss"] != "issuer" || claims["aud"] != "audience" || claims["role"] != "admin" {
		t.Fatalf("unexpected claims: %#v", claims)
	}

	// Ensure PEM loader works
	loaded, err := auth.LoadRSAPrivateKey(string(pemBytes))
	if err != nil || loaded.D.Cmp(key.D) != 0 {
		t.Fatalf("failed to load key via PEM: %v", err)
	}
}
