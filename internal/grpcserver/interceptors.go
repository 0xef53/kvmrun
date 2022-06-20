package grpcserver

import (
	"context"

	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc "google.golang.org/grpc"

	log "github.com/sirupsen/logrus"
)

func unaryLogRequestInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	log.WithFields(grpc_ctxtags.Extract(ctx).Values()).Info("GRPC Request: ", info.FullMethod)

	return handler(ctx, req)
}
