package pubsub_server

import (
	"context"
	"github.com/YuarenArt/PubSubService/pkg/subpub"
	pb "github.com/YuarenArt/PubSubService/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
)

type PubSubServer struct {
	pb.UnimplementedPubSubServer
	sp subpub.SubPub
}

func NewPubSubServer(sp subpub.SubPub) *PubSubServer {
	return &PubSubServer{sp: sp}
}

func (s *PubSubServer) Subscribe(req *pb.SubscribeRequest, stream pb.PubSub_SubscribeServer) error {
	key := req.GetKey()
	if key == "" {
		return status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Подписываемся на события через subpub
	sub, err := s.sp.Subscribe(key, func(msg interface{}) {
		event, ok := msg.(*pb.Event)
		if !ok {
			log.Printf("Received invalid message type: %v", msg)
			return
		}
		if err := stream.Send(event); err != nil {
			log.Printf("Failed to send event to stream: %v", err)
			cancel()
		}
	}, subpub.WithBufferSize(100))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	<-ctx.Done()
	return nil
}

func (s *PubSubServer) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {

	if err := ctx.Err(); err != nil {
		return nil, status.Error(codes.Canceled, "request canceled")
	}

	key := req.GetKey()
	data := req.GetData()
	if key == "" || data == "" {
		return nil, status.Error(codes.InvalidArgument, "key and data cannot be empty")
	}

	event := &pb.Event{Data: data}
	if err := s.sp.Publish(key, event); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
	}

	return &emptypb.Empty{}, nil
}
