# httpc — GoStratum HTTP Client

`httpc` is a composable outbound HTTP client tailored for GoStratum services. It wraps the standard library client with configurable retries, circuit breakers, and pluggable authentication strategies that align with the GoStratum stack.

## Features
- Functional request builder with JSON, form, multipart, and raw payload helpers
- Pluggable auth providers (API Key, Basic, JWT HS256/RS256) plus per-request overrides
- Optional zap-powered retry logging for visibility into backoff attempts
- Exponential backoff with jitter, retryable status codes, and per-request force retry
- Optional host-scoped circuit breaker powered by `github.com/sony/gobreaker`
- Transport middleware chain (retry → breaker → gzip → base) with custom middleware hooks
- Fx module for painless DI/config integration via `configx`
- Safe gzip/deflate handling, idempotency helpers, timeout overrides, and custom middleware injection

## Installation

```bash
go get github.com/gostratum/httpc
```

## Quick Start

### Standalone

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gostratum/httpc"
	"github.com/gostratum/httpc/auth"
)

func main() {
	client, err := httpc.New(
		httpc.WithBaseURL("https://api.example.com"),
		httpc.WithTimeout(8*time.Second),
		httpc.WithAuth(auth.NewAPIKey(auth.APIKeyOptions{Key: "sekret"})),
	)
	if err != nil {
		panic(err)
	}

	resp, err := client.Post(context.Background(), "/v1/items",
		map[string]any{"name": "demo"},
		httpc.WithHeader("X-Correlation-Id", "123"),
	)
	if err != nil {
		panic(err)
	}

	var out map[string]any
	_ = resp.DecodeJSON(&out)
	fmt.Println(out)
}
```

### Fx Integration

```go
package main

import (
	"context"

	"github.com/gostratum/core/configx"
	"github.com/gostratum/httpc"
	httpcfx "github.com/gostratum/httpc/fx"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		fx.Provide(configx.New),
		httpcfx.Module(),
		fx.Provide(fx.Annotate(
			func() httpc.Option { return httpc.WithBaseURL("https://api.example.com") },
			fx.ResultTags(`group:"httpc_options"`),
		)),
		fx.Invoke(func(lc fx.Lifecycle, client httpc.Client) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					_, _ = client.Get(ctx, "/health",
						httpc.WithHeader("X-Tenant", "acme"),
					)
					return nil
				},
			})
		}),
	)
	app.Run()
}
```

## Configuration

The config struct is bindable via `configx` using the `httpc` prefix.

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `env` | string | `dev` | Environment hint (`dev`/`prod`) |
| `base_url` | string | | Optional base URL used for relative requests |
| `timeout` | duration | `10s` | Default client timeout |
| `max_idle_conns` | int | `100` | Transport idle pool size |
| `idle_conn_timeout` | duration | `90s` | Idle connection lifetime |
| `retry_enabled` | bool | `true` | Global retry toggle |
| `retry_max_attempts` | int | `3` | Max attempts (initial attempt + retries) |
| `retry_base_backoff` | duration | `200ms` | Initial backoff |
| `retry_max_backoff` | duration | `2s` | Cap for backoff |
| `retry_on_statuses` | []int | `502,503,504` | Status codes considered retryable |
| `breaker_enabled` | bool | `false` | Enable circuit breaker middleware |
| `api_key.key` | string | | API key secret |
| `api_key.in` | string | `header` | `header` or `query` |
| `api_key.name` | string | `X-API-Key` | Header or query parameter name |
| `basic.username` | string | | Basic auth username |
| `basic.password` | string | | Basic auth password |
| `jwt.alg` | string | `RS256` | `HS256` or `RS256` |
| `jwt.issuer` | string | | `iss` claim |
| `jwt.audience` | string | | `aud` claim |
| `jwt.kid` | string | | Optional key identifier |
| `jwt.ttl` | duration | `60s` | Token lifetime |
| `jwt.hmac_secret` | string | | HS256 secret (literal string or `file:` path) |
| `jwt.private_pem` | string | | RS256 key PEM (literal or `file:` path) |

### Runtime-only options

Functional options augment values that are not part of the serialized config, e.g.:

- `httpc.WithLogger(*zap.Logger)`
- `httpc.WithTransport(http.RoundTripper)` / `httpc.WithHTTPClient`
- `httpc.WithMiddleware(httpc.Middleware)` for custom round-trippers

Additional options include `httpc.WithUserAgent`, `httpc.WithRetry(false, maxAttempts)`, `httpc.WithBreaker(true)`, and `httpc.WithAuth` for setting defaults.

## Retry and Circuit Breaker

- Exponential backoff with jitter (`2^(attempt-1)` scaling within configured bounds).
- Default idempotent methods: GET, HEAD, OPTIONS, PUT, DELETE. Use `httpc.WithRetryForce()` on per-request basis to retry e.g. POST.
- Retry on transport errors and configured status codes.
- Circuit breaker (sony/gobreaker) keyed by request host, configurable via `WithBreakerManager`.

## Security Notes

- JWT provider supports HS256 and RS256 with automatic short-lived (`TTL`) tokens and optional `kid`.
- Multipart helpers buffer payloads in memory; supply your own `ReqOption` for streaming if needed.
- Provide custom middleware if you need header/query redaction in logs today (native support is planned).

## Testing

Unit tests cover authentication helpers, request builders, and retry behaviour. Run them with:

```bash
go test ./...
```

## Examples

See the `examples/` directory for:

- `basic/` — Fx wiring example using `httpcfx.Module`.
- `standalone/` — Direct usage with functional options.

## License

Apache 2.0 (pending confirmation within GoStratum repos).
