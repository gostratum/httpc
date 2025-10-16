package httpcfx

import (
	"github.com/gostratum/httpc"
	"go.uber.org/fx"
)

// Module exposes the httpc client via an Fx module.
func Module() fx.Option {
	return fx.Module("httpc",
		fx.Provide(
			httpc.NewConfigFx,
			httpc.NewFx,
		),
	)
}
