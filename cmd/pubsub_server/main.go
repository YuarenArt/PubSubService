package main

import (
	"context"
	"github.com/YuarenArt/PubSubService/internal/config"
	"github.com/YuarenArt/PubSubService/internal/logging"
	"github.com/YuarenArt/PubSubService/internal/pubsub_server"
	"github.com/YuarenArt/PubSubService/pkg/subpub"
	pb "github.com/YuarenArt/PubSubService/proto"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.NewConfig()
	cfg.LogFile = "logs/server.log"

	logger := logging.NewLogger(cfg)
	logger.Info("Starting PubSub gRPC service", "addr", cfg.GRPCAddr, "log_type", cfg.LogType)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		os.Exit(1)
	}
	defer lis.Close()

	sp := subpub.NewSubPub()
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(logging.UnaryServerInterceptor(logger)),
		grpc.StreamInterceptor(logging.StreamServerInterceptor(logger)),
	)

	srv := pubsub_server.NewPubSubServer(sp, logger)
	pb.RegisterPubSubServer(grpcServer, srv)

	go func() {
		logger.Info("gRPC server started", "addr", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down gRPC server")
	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sp.Close(ctx); err != nil {
		logger.Error("Error closing SubPub", "error", err)
	}

	logger.Info("Server stopped gracefully")
}
