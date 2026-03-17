package database

import "context"

type Adapter interface {
	Reader
	Writer

	Connect(ctx context.Context) error
	Close(ctx context.Context) error

	IsReady() bool
}
