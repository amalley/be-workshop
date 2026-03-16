package database

import (
	"context"

	"github.com/AMalley/be-workshop/ch-3/api/database/scylla"
	"github.com/AMalley/be-workshop/ch-3/models"
)

type DatabaseAdpater interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error

	IsReady() bool

	InsertStats(ctx context.Context, stats models.WikiStatsModel) error
	GetStats(ctx context.Context) (models.WikiStatsCounts, error)

	GetUser(ctx context.Context, username string) (models.User_DB, bool, error)
	CreateUser(ctx context.Context, username, password string) error
	DeleteUser(ctx context.Context, username string) error

	GetUserByID(ctx context.Context, userID string) (models.User_DB, bool, error)
}

var _ DatabaseAdpater = &scylla.ScyllaDatabaseAdapter{}
