package client

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/internal/appconf"
)

type appConfKey struct{}

func AppendAppConfToContext(ctx context.Context, appConf *appconf.Config) context.Context {
	return context.WithValue(ctx, appConfKey{}, appConf)
}

func AppConfFromContext(ctx context.Context) (*appconf.Config, error) {
	if v := ctx.Value(appConfKey{}); v != nil {
		if c, ok := v.(*appconf.Config); ok {
			return c, nil
		} else {
			return nil, fmt.Errorf("invalid appConf interface: t = %T", v)
		}
	}

	return nil, fmt.Errorf("appConf value not found in context")
}
