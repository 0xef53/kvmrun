package metadata

import "context"

type taskMetadataKey struct{}

func AppendToContext(ctx context.Context, md interface{}) context.Context {
	return context.WithValue(ctx, taskMetadataKey{}, md)
}

func FromContext(ctx context.Context) (interface{}, bool) {
	md := ctx.Value(taskMetadataKey{})

	if md == nil {
		return nil, false
	}

	return md, true
}
