package database

import (
	"context"

	"github.com/AMalley/be-workshop/ch-3/models"
)

type DatabaseAdpater interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error

	IsReady() bool

	Insert(stats models.WikiStatsModel) error
}
