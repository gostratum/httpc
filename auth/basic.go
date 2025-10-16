package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
)

// BasicOptions configure basic auth provider.
type BasicOptions struct {
	Username string
	Password string
}

// NewBasic constructs an AuthProvider that sets HTTP Basic credentials.
func NewBasic(opts BasicOptions) AuthProvider {
	return &basicProvider{
		username: opts.Username,
		password: opts.Password,
	}
}

type basicProvider struct {
	username string
	password string
}

func (p *basicProvider) Apply(req *http.Request) error {
	if p.username == "" {
		return fmt.Errorf("username is required for basic auth")
	}
	token := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))
	req.Header.Set("Authorization", "Basic "+token)
	return nil
}

func (p *basicProvider) Name() string {
	return "basic"
}
