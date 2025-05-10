package pubsub_server

import (
	"context"
	"fmt"
	"github.com/YuarenArt/PubSubService/internal/logging"
	"github.com/YuarenArt/PubSubService/pkg/subpub"
	pb "github.com/YuarenArt/PubSubService/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PubSubServer implements the gRPC PubSub service with a Publisher-Subscriber backend.
type PubSubServer struct {
	pb.UnimplementedPubSubServer
	sp     subpub.SubPub
	logger logging.Logger
}

// NewPubSubServer creates a new PubSubServer instance with the given SubPub backend.
func NewPubSubServer(sp subpub.SubPub, logger logging.Logger) *PubSubServer {
	return &PubSubServer{
		sp:     sp,
		logger: logger,
	}
}

// Subscribe handles subscription requests, streaming events to the client for a given key.
func (s *PubSubServer) Subscribe(req *pb.SubscribeRequest, stream pb.PubSub_SubscribeServer) error {
	key := req.GetKey()
	if key == "" {
		s.logger.Error("Subscription rejected: empty key")
		return status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Log subscription attempt
	s.logger.Info("Subscribing to key", "key", key)

	sub, err := s.sp.Subscribe(key, func(msg interface{}) {
		event, ok := msg.(*pb.Event)
		if !ok {
			s.logger.Error("Received invalid message type", "type", fmt.Sprintf("%T", msg))
			return
		}
		if err := stream.Send(event); err != nil {
			s.logger.Error("Failed to send event to stream", "key", key, "error", err)
			cancel()
		} else {
			s.logger.Info("Sent event to stream", "key", key, "data", event.Data)
		}
	}, subpub.WithBufferSize(100))
	if err != nil {
		s.logger.Error("Failed to subscribe", "key", key, "error", err)
		return status.Errorf(codes.Internal, "failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	s.logger.Info("Subscription established", "key", key)
	<-ctx.Done()
	s.logger.Info("Subscription closed", "key", key)
	return nil
}

// Publish handles publication requests, sending an event to all subscribers of the given key.
func (s *PubSubServer) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {
	if err := ctx.Err(); err != nil {
		s.logger.Error("Publish rejected: context error", "error", err)
		return nil, status.Error(codes.Canceled, "request canceled")
	}

	key := req.GetKey()
	data := req.GetData()
	if key == "" || data == "" {
		s.logger.Error("Publish rejected: invalid input", "key", key, "data", data)
		return nil, status.Error(codes.InvalidArgument, "key and data cannot be empty")
	}

	// Log publication attempt
	s.logger.Info("Publishing to key", "key", key, "data", data)

	event := &pb.Event{Data: data}
	if err := s.sp.Publish(key, event); err != nil {
		s.logger.Error("Failed to publish", "key", key, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
	}

	s.logger.Info("Published successfully", "key", key, "data", data)
	return &emptypb.Empty{}, nil
}
