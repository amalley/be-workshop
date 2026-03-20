package database

import (
	"context"

	"github.com/amalley/be-workshop/ch-3/models"
	"github.com/gocql/gocql"
)

type DatabaseAdapter interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error

	IsReady() bool

	InsertStats(ctx context.Context, stats models.WikiStatsModel) error
	GetStats(ctx context.Context) (models.WikiStatsCounts, error)

	GetUser(ctx context.Context, username string) (models.User, bool, error)
	CreateUser(ctx context.Context, username, password string) error
	DeleteUser(ctx context.Context, userID gocql.UUID) error

	GetUserByID(ctx context.Context, userID gocql.UUID) (models.User, bool, error)
}
