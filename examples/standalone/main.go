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

	resp, err := client.Get(context.Background(), "/v1/health")
	if err != nil {
		panic(err)
	}

	fmt.Println("status:", resp.StatusCode())
}
