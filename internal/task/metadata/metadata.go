package metadata

import "context"

type taskMetadataKey struct{}

// AppendToContext returns a new context with the given value
// written into original ctx as metadata.
func AppendToContext(ctx context.Context, md interface{}) context.Context {
	return context.WithValue(ctx, taskMetadataKey{}, md)
}

// FromContext returns the metadata from ctx if it exists.
func FromContext(ctx context.Context) (interface{}, bool) {
	md := ctx.Value(taskMetadataKey{})

	if md == nil {
		return nil, false
	}

	return md, true
}
