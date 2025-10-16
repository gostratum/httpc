package main

import (
	"context"
	"fmt"
	"time"

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
			func() httpc.Option {
				return httpc.WithBaseURL("https://api.example.com")
			},
			fx.ResultTags(`group:"httpc_options"`),
		)),
		fx.Invoke(func(lc fx.Lifecycle, client httpc.Client) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()
					resp, err := client.Get(ctx, "/ping")
					if err != nil {
						fmt.Println("call failed:", err)
						return nil
					}
					fmt.Println("status:", resp.StatusCode())
					return nil
				},
			})
		}),
	)
	app.Run()
}
