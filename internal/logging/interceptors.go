package logging

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"time"
)

// UnaryServerInterceptor logs unary gRPC requests with method, duration, status, and errors.
func UnaryServerInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		start := time.Now()

		resp, err = handler(ctx, req)

		st, _ := status.FromError(err)
		logger.Info("gRPC unary request",
			"method", info.FullMethod,
			"duration", time.Since(start),
			"code", st.Code().String(),
			"error", err,
		)
		return resp, err
	}
}

// StreamServerInterceptor logs streaming gRPC requests with method, duration, status, and errors.
func StreamServerInterceptor(logger Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		err := handler(srv, ss)

		st, _ := status.FromError(err)
		logger.Info("gRPC stream request",
			"method", info.FullMethod,
			"duration", time.Since(start),
			"isClientStream", info.IsClientStream,
			"isServerStream", info.IsServerStream,
			"code", st.Code().String(),
			"error", err,
		)
		return err
	}
}
