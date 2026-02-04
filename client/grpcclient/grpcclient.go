package grpc_client

import (
	"context"
	"io"
	"strings"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	grpcclient "github.com/0xef53/go-grpc/client"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v3"
)

func init() {
	logger := log.New()

	logger.SetOutput(io.Discard)

	grpcclient.SetLogger(log.NewEntry(logger))
}

func CommandGRPC(ctx context.Context, c *cli.Command, fns ...func(context.Context, string, *cli.Command, *grpc_interfaces.Kvmrun) error) error {
	requiredArgs := func() (n int) {
		for _, v := range strings.Fields(c.ArgsUsage) {
			if !strings.HasPrefix(v, "[") {
				n++
			}
		}
		return n
	}()

	if c.Args().Len() < requiredArgs {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}

	vmname := c.Args().First()

	return executeGRPC(ctx, func(client *grpc_interfaces.Kvmrun) error {
		for _, fn := range fns {
			if err := fn(ctx, vmname, c, client); err != nil {
				return err
			}
		}

		return nil
	})
}

func KvmrunGRPC(ctx context.Context, fn func(*grpc_interfaces.Kvmrun) error) error {
	return executeGRPC(ctx, fn)
}

func executeGRPC(ctx context.Context, fn func(*grpc_interfaces.Kvmrun) error) error {
	appConf, err := AppConfFromContext(ctx)
	if err != nil {
		return err
	}

	conn, err := grpcclient.NewSecureConnection("unix:@/run/kvmrund.sock", appConf.TLSConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	return fn(grpc_interfaces.NewKvmrunInterface(conn))
}
