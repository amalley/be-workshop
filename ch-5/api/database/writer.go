package database

import (
	"context"

	"github.com/amalley/be-workshop/ch-5/models"
	"github.com/gocql/gocql"
)

type Writer interface {
	InsertStats(ctx context.Context, stats models.WikiStatsModel) error
	CreateUser(ctx context.Context, username, password string) error
	DeleteUser(ctx context.Context, userID gocql.UUID) error
}
