package stream

import "context"

// Adapter defines the interface need for connenting and consuming a data stream.
type Adapter interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	Consume(ctx context.Context) error
}
