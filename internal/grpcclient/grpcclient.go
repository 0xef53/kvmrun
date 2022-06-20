package grpcclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_credentials "google.golang.org/grpc/credentials"
	grpc_metadata "google.golang.org/grpc/metadata"
	grpc_status "google.golang.org/grpc/status"
)

func NewConn(addr string, tlsConfig *tls.Config, withRetries bool) (*grpc.ClientConn, error) {
	switch len(strings.Split(addr, ":")) {
	case 1:
		addr = addr + ":9393"
	case 2:
	default:
		return nil, fmt.Errorf("invalid addr string: %s", addr)
	}

	var options []grpc.DialOption

	if tlsConfig == nil {
		options = append(options, grpc.WithInsecure())
	} else {
		options = append(options, grpc.WithTransportCredentials(grpc_credentials.NewTLS(tlsConfig)))
	}

	interceptors := []grpc.UnaryClientInterceptor{
		withOutgoingContext,
	}

	if withRetries {
		interceptors = append(interceptors, withRequestRetries)
	}

	options = append(options, grpc.WithChainUnaryInterceptor(interceptors...))

	return grpc.Dial(addr, options...)
}

/*
	if tlsConfig != nil {
		return grpc.Dial(addr, grpc.WithTransportCredentials(grpc_credentials.NewTLS(tlsConfig)), withClientUnaryInterceptor())
	}

	// Insecure
	return grpc.Dial(addr, grpc.WithInsecure(), withClientUnaryInterceptor())
*/

func withOutgoingContext(ctx context.Context, method string, req, resp interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var outmd grpc_metadata.MD

	if md, ok := grpc_metadata.FromOutgoingContext(ctx); ok {
		outmd = md.Copy()
	}

	ctx = grpc_metadata.NewOutgoingContext(context.Background(), outmd)

	return invoker(ctx, method, req, resp, cc, opts...)
}

func withRequestRetries(ctx context.Context, method string, req, resp interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var err error

	for attempt := 0; attempt < 10; attempt++ {
		err = invoker(ctx, method, req, resp, cc, opts...)

		if grpc_status.Code(err) == grpc_codes.Unavailable {
			time.Sleep(7 * time.Second)
			continue
		}

		break
	}

	return err
}

func withClientUnaryInterceptor() grpc.DialOption {
	return grpc.WithUnaryInterceptor(
		func(ctx context.Context, method string, req, resp interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			var outmd grpc_metadata.MD

			if md, ok := grpc_metadata.FromOutgoingContext(ctx); ok {
				outmd = md.Copy()
			}

			ctx = grpc_metadata.NewOutgoingContext(context.Background(), outmd)

			var err error

			for attempt := 0; attempt < 10; attempt++ {
				err = invoker(ctx, method, req, resp, cc, opts...)

				if grpc_status.Code(err) == grpc_codes.Unavailable {
					time.Sleep(7 * time.Second)
					continue
				}

				break
			}

			return err
		},
	)
}
