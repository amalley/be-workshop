package database

import (
	"context"

	"github.com/amalley/be-workshop/ch-9/models"
	"github.com/gocql/gocql"
)

type Reader interface {
	GetUser(ctx context.Context, username string) (models.User, bool, error)
	GetUserByID(ctx context.Context, userID gocql.UUID) (models.User, bool, error)
	GetStats(ctx context.Context) (models.WikiStatsCounts, error)
}
