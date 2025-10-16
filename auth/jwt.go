package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTOptions configures the JWT auth provider.
type JWTOptions struct {
	Alg        string
	Issuer     string
	Audience   string
	Kid        string
	TTL        time.Duration
	HMACSecret []byte
	PrivateKey *rsa.PrivateKey
}

// JWTOption is a functional option applied to the JWT provider.
type JWTOption func(*jwtProvider)

// WithClaims registers a callback to mutate claims prior to signing.
func WithClaims(fn func(jwt.MapClaims)) JWTOption {
	return func(p *jwtProvider) {
		p.claimsMutators = append(p.claimsMutators, fn)
	}
}

// WithClock overrides the time source (useful for testing).
func WithClock(clock func() time.Time) JWTOption {
	return func(p *jwtProvider) {
		if clock != nil {
			p.clock = clock
		}
	}
}

// NewJWT constructs a JWT-based bearer auth provider.
func NewJWT(opts JWTOptions, more ...JWTOption) (AuthProvider, error) {
	alg := strings.ToUpper(strings.TrimSpace(opts.Alg))
	if alg == "" {
		alg = "RS256"
	}

	if opts.TTL <= 0 {
		opts.TTL = 60 * time.Second
	}

	p := &jwtProvider{
		options: opts,
		alg:     alg,
		ttl:     opts.TTL,
		clock:   time.Now,
		keyID:   opts.Kid,
	}

	for _, opt := range more {
		opt(p)
	}

	switch alg {
	case "HS256":
		if len(opts.HMACSecret) == 0 {
			return nil, fmt.Errorf("hs256 requires hmac secret")
		}
		p.signingMethod = jwt.SigningMethodHS256
	case "RS256":
		if opts.PrivateKey == nil {
			return nil, fmt.Errorf("rs256 requires private key")
		}
		p.signingMethod = jwt.SigningMethodRS256
	default:
		return nil, fmt.Errorf("unsupported jwt alg %q", alg)
	}

	return p, nil
}

type jwtProvider struct {
	options        JWTOptions
	alg            string
	ttl            time.Duration
	signingMethod  jwt.SigningMethod
	claimsMutators []func(jwt.MapClaims)
	clock          func() time.Time
	keyID          string
}

func (p *jwtProvider) Apply(req *http.Request) error {
	now := p.clock()
	exp := now.Add(p.ttl)

	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"nbf": now.Unix(),
	}

	if p.options.Issuer != "" {
		claims["iss"] = p.options.Issuer
	}
	if p.options.Audience != "" {
		claims["aud"] = p.options.Audience
	}

	for _, mutate := range p.claimsMutators {
		mutate(claims)
	}

	if _, ok := claims["jti"]; !ok {
		if jti := randomJTI(); jti != "" {
			claims["jti"] = jti
		}
	}

	token := jwt.NewWithClaims(p.signingMethod, claims)
	if p.keyID != "" {
		token.Header["kid"] = p.keyID
	}

	var signed string
	var err error

	switch p.alg {
	case "HS256":
		signed, err = token.SignedString(p.options.HMACSecret)
	case "RS256":
		signed, err = token.SignedString(p.options.PrivateKey)
	default:
		return fmt.Errorf("unsupported jwt alg %q", p.alg)
	}
	if err != nil {
		return fmt.Errorf("sign jwt: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+signed)
	return nil
}

func (p *jwtProvider) Name() string {
	return "jwt:" + p.alg
}

func randomJTI() string {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
