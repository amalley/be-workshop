package stream

import "context"

// StreamAdapter defines the interface need for connenting and consuming a data stream.
type StreamAdapter interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	Consume(ctx context.Context) error
}
