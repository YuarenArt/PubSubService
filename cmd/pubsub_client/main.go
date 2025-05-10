package main

import (
	"context"
	"github.com/YuarenArt/PubSubService/internal/config"
	"github.com/YuarenArt/PubSubService/internal/logging"
	"github.com/YuarenArt/PubSubService/internal/pubsub_client"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg := config.NewConfig()
	cfg.LogFile = "logs/client.log"
	logger := logging.NewLogger(cfg)
	logger.Info("Starting PubSub gRPC client", "addr", cfg.GRPCAddr)

	cl, err := pubsub_client.NewPubSubClient(cfg.GRPCAddr, logger)
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}
	defer cl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		cl.Run(ctx)
	}()

	// Handle shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down client")
	cancel()

	// Wait for Run to exit cleanly
	wg.Wait()
	logger.Info("Client stopped gracefully")
}
