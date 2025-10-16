package httpc

import (
	"github.com/gostratum/core/configx"
	"github.com/gostratum/core/logx"
	"go.uber.org/fx"
)

// FxConfigParams wires config loading via fx.
type FxConfigParams struct {
	fx.In

	Loader configx.Loader
}

// FxParams captures dependencies resolved via fx when constructing a client.
type FxParams struct {
	fx.In

	Config        Config
	Logger        logx.Logger `optional:"true"`
	CustomOptions []Option    `group:"httpc_options"`
}

// NewFx loads configuration via configx and constructs a Client suitable for fx.
func NewFx(params FxParams) (Client, error) {
	var opts []Option

	opts = append(opts, WithConfig(params.Config))

	if params.Logger != nil {
		opts = append(opts, WithLogger(params.Logger))
	}
	if len(params.CustomOptions) > 0 {
		opts = append(opts, params.CustomOptions...)
	}

	return New(opts...)
}

// NewConfigFx binds the Config using configx.
func NewConfigFx(params FxConfigParams) (Config, error) {
	return NewConfig(params.Loader)
}
