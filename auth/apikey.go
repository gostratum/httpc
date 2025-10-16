package auth

import (
	"fmt"
	"net/http"
	"strings"
)

// APIKeyOptions configures the API key auth provider.
type APIKeyOptions struct {
	Key  string
	In   string
	Name string
}

// NewAPIKey constructs an AuthProvider that injects an API key either via a
// header or query parameter.
func NewAPIKey(opts APIKeyOptions) AuthProvider {
	location := strings.ToLower(strings.TrimSpace(opts.In))
	if location == "" {
		location = "header"
	}
	name := opts.Name
	if name == "" {
		name = "X-API-Key"
	}
	return &apiKeyProvider{
		key:      opts.Key,
		location: location,
		name:     name,
	}
}

type apiKeyProvider struct {
	key      string
	location string
	name     string
}

func (p *apiKeyProvider) Apply(req *http.Request) error {
	if p.key == "" {
		return fmt.Errorf("api key is empty")
	}

	switch p.location {
	case "query":
		u := req.URL
		if u == nil {
			return fmt.Errorf("request URL is nil")
		}
		query := u.Query()
		query.Set(p.name, p.key)
		u.RawQuery = query.Encode()
	case "header", "":
		req.Header.Set(p.name, p.key)
	default:
		return fmt.Errorf("unsupported api key location: %s", p.location)
	}

	return nil
}

func (p *apiKeyProvider) Name() string {
	switch p.location {
	case "query":
		return fmt.Sprintf("api-key-query:%s", p.name)
	default:
		return fmt.Sprintf("api-key-header:%s", p.name)
	}
}
