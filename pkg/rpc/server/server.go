package rpcserver

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"net"
	"net/http"
	"runtime"

	"golang.org/x/net/http2"
	"golang.org/x/sync/errgroup"

	"github.com/0xef53/kvmrun/pkg/rpc/common"
)

func serve(ctx context.Context, h http.Handler, addr string, tlsConfig *tls.Config) error {
	srv := http.Server{Handler: h}

	var listener net.Listener

	if tlsConfig == nil {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		listener = l
	} else {
		http2.ConfigureServer(&srv, &http2.Server{})

		l, err := tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		listener = l
	}
	defer listener.Close()

	idleConnsClosed := make(chan struct{})

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
		close(idleConnsClosed)
	}()

	if err := srv.Serve(listener); err != http.ErrServerClosed {
		// Error starting or closing listener
		return err
	}

	<-idleConnsClosed

	return nil
}

func ServeTls(ctx context.Context, h http.Handler, addrs []string, crt, key string) error {
	tlsConfig, err := rpccommon.TlsConfig(crt, key)
	if err != nil {
		return err
	}
	tlsConfig.Rand = rand.Reader

	group1, ctx1 := errgroup.WithContext(ctx)

	for _, addr := range addrs {
		_addr := addr
		group1.Go(func() error { return serve(ctx1, h, _addr, tlsConfig) })
	}

	return group1.Wait()
}

func ServeUnixSocket(ctx context.Context, h http.Handler, sockpath string) error {
	if sockpath[0] == '@' && runtime.GOOS == "linux" {
		sockpath = sockpath + string(0)
	}

	srv := http.Server{Handler: h}

	listener, err := net.Listen("unix", sockpath)
	if err != nil {
		return err
	}
	defer listener.Close()

	idleConnsClosed := make(chan struct{})

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
		close(idleConnsClosed)
	}()

	if err := srv.Serve(listener); err != http.ErrServerClosed {
		// Error starting or closing listener
		return err
	}

	<-idleConnsClosed

	return nil
}
