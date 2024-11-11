package main

import (
	"context"
	"log"
	"os"
	"sync"

	gateway "github.com/pramodrj07/api-gateway/gateway"
)

func main() {
	log := log.New(os.Stdout, "API-gateway", log.LstdFlags)

	ctx := context.Background()
	lock := sync.Mutex{}

	gateway := gateway.NewGateway(ctx, &lock, log)
	gateway.Run()
}
