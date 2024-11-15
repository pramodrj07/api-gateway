package main

import (
	"context"
	"log"
	"os"
	"sync"

	gateway "github.com/pramodrj07/api-gateway/gateway"
)

const (
	configPath = "config.yaml"
	APIgateway = "API-gateway"
)

func main() {
	log := log.New(os.Stdout, APIgateway, log.LstdFlags)

	ctx := context.Background()
	lock := sync.Mutex{}

	gateway := gateway.NewGateway(ctx, &lock, configPath, log)
	gateway.Run()
}
