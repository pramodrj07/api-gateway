package main

import (
	"context"
	"log"
	"os"

	gateway "github.com/pramodrj07/api-gateway/gateway"
)

func main() {
	log := log.New(os.Stdout, "API-gateway", log.LstdFlags)
	ctx := context.Background()
	gateway := gateway.NewGateway(ctx, log)
	gateway.Run()
}
