package auth

import (
	"net/http"
)

// AuthProvider decorates outgoing requests with authentication metadata.
type AuthProvider interface {
	Apply(req *http.Request) error
	Name() string
}

// ProviderFunc adapts a function into an AuthProvider.
type ProviderFunc func(req *http.Request) error

func (f ProviderFunc) Apply(req *http.Request) error { return f(req) }
func (f ProviderFunc) Name() string                  { return "provider-func" }
