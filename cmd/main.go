package main

import (
	"context"
	"log"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	gateway "github.com/pramodrj07/api-gateway/gateway"
)

const (
	configPath = "config.yaml"
	APIgateway = "API-gateway"
)

func main() {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.EncoderConfig.TimeKey = "timestamp"
	loggerConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC1123Z)
	loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	logger, err := loggerConfig.Build()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	lock := sync.Mutex{}

	gateway := gateway.NewGateway(ctx, &lock, configPath, logger)
	gateway.Run()
}
