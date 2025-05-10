// cmd/client/main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/YuarenArt/PubSubService/internal/config"
	"github.com/YuarenArt/PubSubService/internal/logging"
	pb "github.com/YuarenArt/PubSubService/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {

	cfg := config.NewConfig()
	cfg.LogFile = "logs/client.log"

	logger := logging.NewLogger(cfg)
	logger.Info("Starting PubSub gRPC client", "addr", cfg.GRPCAddr)

	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to gRPC server", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewPubSubClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleConsoleInput(ctx, client, logger)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down client")
	cancel()
	time.Sleep(500 * time.Millisecond)
}

func handleConsoleInput(ctx context.Context, client pb.PubSubClient, logger logging.Logger) {
	reader := bufio.NewReader(os.Stdin)
	activeSubscriptions := make(map[string]context.CancelFunc)
	var mu sync.Mutex

	fmt.Println("PubSub Client. Available commands: subscribe <key>, unsubscribe <key>, publish <key> <data>, exit")

	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Failed to read input", "error", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}

		cmd := args[0]
		switch cmd {
		case "subscribe":
			if len(args) != 2 {
				fmt.Println("Usage: subscribe <key>")
				continue
			}
			key := args[1]
			mu.Lock()
			if _, exists := activeSubscriptions[key]; exists {
				fmt.Printf("Already subscribed to key: %s\n", key)
				mu.Unlock()
				continue
			}
			subCtx, subCancel := context.WithCancel(ctx)
			activeSubscriptions[key] = subCancel
			mu.Unlock()
			go subscribeToKey(subCtx, client, key, logger)

		case "unsubscribe":
			if len(args) != 2 {
				fmt.Println("Usage: unsubscribe <key>")
				continue
			}
			key := args[1]
			mu.Lock()
			if cancel, exists := activeSubscriptions[key]; exists {
				cancel()
				delete(activeSubscriptions, key)
				fmt.Printf("Unsubscribed from key: %s\n", key)
			} else {
				fmt.Printf("Not subscribed to key: %s\n", key)
			}
			mu.Unlock()

		case "publish":
			if len(args) < 3 {
				fmt.Println("Usage: publish <key> <data>")
				continue
			}
			key := args[1]
			data := strings.Join(args[2:], " ")
			req := &pb.PublishRequest{Key: key, Data: data}
			_, err := client.Publish(ctx, req)
			if err != nil {
				if s, ok := status.FromError(err); ok {
					logger.Error("Publish failed", "key", key, "code", s.Code(), "error", s.Message())
				} else {
					logger.Error("Publish failed", "key", key, "error", err)
				}
			} else {
				fmt.Printf("Published successfully to key: %s, data: %s\n", key, data)
			}

		case "exit":
			fmt.Println("Exiting...")
			return

		default:
			fmt.Println("Unknown command. Available: subscribe, unsubscribe, publish, exit")
		}
	}
}

func subscribeToKey(ctx context.Context, client pb.PubSubClient, key string, logger logging.Logger) {
	stream, err := client.Subscribe(ctx, &pb.SubscribeRequest{Key: key})
	if err != nil {
		if s, ok := status.FromError(err); ok {
			logger.Error("Failed to subscribe", "key", key, "code", s.Code(), "error", s.Message())
		} else {
			logger.Error("Failed to subscribe", "key", key, "error", err)
		}
		return
	}
	fmt.Printf("Subscribed to key: %s\n", key)

	for {
		event, err := stream.Recv()
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
				logger.Info("Subscription closed", "key", key)
			} else if s, ok := status.FromError(err); ok {
				logger.Error("Subscription error", "key", key, "code", s.Code(), "error", s.Message())
			} else {
				logger.Error("Subscription error", "key", key, "error", err)
			}
			return
		}
		fmt.Printf("[Event on %s] %s\n", key, event.Data)
	}
}
