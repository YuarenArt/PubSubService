package pubsub_client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/YuarenArt/PubSubService/internal/logging"
	pb "github.com/YuarenArt/PubSubService/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"os"
	"strings"
	"sync"
)

var (
	errExitCommand = fmt.Errorf("exit command received") // errExitCommand is a sentinel error indicating the exit command was received.
)

// PubSubClient encapsulates the gRPC client for the PubSub service.
type PubSubClient struct {
	client pb.PubSubClient
	logger logging.Logger
	conn   *grpc.ClientConn
}

// subscriptionStore manages active subscriptions with their cancellation functions.
type subscriptionStore struct {
	subscriptions map[string]context.CancelFunc
	mu            sync.Mutex
}

// NewPubSubClient creates a new PubSubClient instance connected to the specified gRPC server address.
// Returns an error if the connection fails.
func NewPubSubClient(addr string, logger logging.Logger) (*PubSubClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &PubSubClient{
		client: pb.NewPubSubClient(conn),
		logger: logger,
		conn:   conn,
	}, nil
}

// Run starts the client, reading commands from stdin and processing them until the context is canceled or exit command is received.
func (c *PubSubClient) Run(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)
	store := &subscriptionStore{
		subscriptions: make(map[string]context.CancelFunc),
	}

	c.logger.Info("PubSub client started", "commands", "subscribe <key>, unsubscribe <key>, publish <key> <data>, exit")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Client context canceled, stopping")
			store.cancelAll()
			return
		default:
			if err := c.processCommand(ctx, reader, store); err != nil {
				if errors.Is(err, errExitCommand) {
					store.cancelAll()
					return
				}
				c.logger.Error("Failed to process command", "error", err)
			}
		}
	}
}

// processCommand reads a single command from the reader and processes it.
// Returns errExitCommand for the exit command, or other errors for invalid input.
func (c *PubSubClient) processCommand(ctx context.Context, reader *bufio.Reader, store *subscriptionStore) error {
	// Read input line
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// Parse command
	args := strings.Fields(strings.TrimSpace(line))
	if len(args) == 0 {
		return nil
	}

	cmd := args[0]
	switch cmd {
	case "subscribe":
		return c.handleSubscribeCommand(ctx, args, store)
	case "unsubscribe":
		return c.handleUnsubscribeCommand(args, store)
	case "publish":
		return c.handlePublishCommand(ctx, args)
	case "exit":
		c.logger.Info("Exit command received")
		return errExitCommand
	default:
		c.logger.Warn("Unknown command", "command", cmd, "expected", "subscribe, unsubscribe, publish, exit")
		return nil
	}
}

// handleSubscribeCommand processes the subscribe command.
// Expects args to contain ["subscribe", "<key>"].
func (c *PubSubClient) handleSubscribeCommand(ctx context.Context, args []string, store *subscriptionStore) error {
	if len(args) != 2 {
		c.logger.Warn("Invalid subscribe command format", "args", args)
		return nil
	}

	key := args[1]
	if key == "" {
		c.logger.Warn("Subscribe command rejected: empty key")
		return nil
	}

	// Check if already subscribed
	store.mu.Lock()
	if _, exists := store.subscriptions[key]; exists {
		c.logger.Info("Already subscribed to key", "key", key)
		store.mu.Unlock()
		return nil
	}

	// Create subscription context and store cancellation function
	subCtx, subCancel := context.WithCancel(ctx)
	store.subscriptions[key] = subCancel
	store.mu.Unlock()

	// Start subscription in a separate goroutine
	go c.subscribeToKey(subCtx, key)
	c.logger.Info("Subscribed to key", "key", key)
	return nil
}

// handleUnsubscribeCommand processes the unsubscribe command.
// Expects args to contain ["unsubscribe", "<key>"].
func (c *PubSubClient) handleUnsubscribeCommand(args []string, store *subscriptionStore) error {
	if len(args) != 2 {
		c.logger.Warn("Invalid unsubscribe command format", "args", args)
		return nil
	}

	key := args[1]
	store.mu.Lock()
	defer store.mu.Unlock()

	if cancel, exists := store.subscriptions[key]; exists {
		cancel()
		delete(store.subscriptions, key)
		c.logger.Info("Unsubscribed from key", "key", key)
	} else {
		c.logger.Info("Not subscribed to key", "key", key)
	}
	return nil
}

// handlePublishCommand processes the publish command.
// Expects args to contain ["publish", "<key>", "<data>", ...].
func (c *PubSubClient) handlePublishCommand(ctx context.Context, args []string) error {
	if len(args) < 3 {
		c.logger.Warn("Invalid publish command format", "args", args)
		return nil
	}

	key := args[1]
	data := strings.Join(args[2:], " ")
	if key == "" || data == "" {
		c.logger.Warn("Publish command rejected: empty key or data", "key", key, "data", data)
		return nil
	}

	// Publish the message
	req := &pb.PublishRequest{Key: key, Data: data}
	_, err := c.client.Publish(ctx, req)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			return status.Errorf(s.Code(), "publish failed: %s", s.Message())
		}
		return err
	}

	c.logger.Info("Published successfully", "key", key, "data", data)
	return nil
}

// subscribeToKey handles subscription to a specific key, streaming events and logging them.
func (c *PubSubClient) subscribeToKey(ctx context.Context, key string) {
	stream, err := c.client.Subscribe(ctx, &pb.SubscribeRequest{Key: key})
	if err != nil {
		c.logSubscribeError(key, err)
		return
	}

	// Process incoming events
	for {
		event, err := stream.Recv()
		if err != nil {
			c.logStreamError(key, err)
			return
		}
		c.logger.Info("Received event", "key", key, "data", event.Data)
	}
}

// logSubscribeError logs errors that occur during subscription setup.
func (c *PubSubClient) logSubscribeError(key string, err error) {
	if s, ok := status.FromError(err); ok {
		c.logger.Error("Failed to subscribe", "key", key, "code", s.Code(), "error", s.Message())
	} else {
		c.logger.Error("Failed to subscribe", "key", key, "error", err)
	}
}

// logStreamError logs errors that occur during event streaming.
func (c *PubSubClient) logStreamError(key string, err error) {
	if s, ok := status.FromError(err); ok {
		if s.Code() == codes.Canceled {
			c.logger.Info("Subscription closed", "key", key)
		} else {
			c.logger.Error("Subscription error", "key", key, "code", s.Code(), "error", s.Message())
		}
	} else {
		c.logger.Error("Subscription error", "key", key, "error", err)
	}
}

// cancelAll cancels all active subscriptions.
func (s *subscriptionStore) cancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, cancel := range s.subscriptions {
		cancel()
		delete(s.subscriptions, key)
	}
}

// Close closes the gRPC connection.
func (c *PubSubClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
