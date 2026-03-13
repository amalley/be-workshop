package scylla

import (
	"context"
	"log/slog"

	"github.com/AMalley/be-workshop/ch-3/models"
)

type ScyllaDatabaseAdapter struct {
	logger *slog.Logger
	host   string
}

func NewScyllaDatabaseAdapter(logger *slog.Logger, host string) *ScyllaDatabaseAdapter {
	return &ScyllaDatabaseAdapter{
		logger: logger.With(slog.String("src", "ScyllaDatabaseAdapter")),
		host:   host,
	}
}

func (s *ScyllaDatabaseAdapter) Connect(ctx context.Context) error {
	return nil
}

func (s *ScyllaDatabaseAdapter) Close(ctx context.Context) error {
	return nil
}

func (s *ScyllaDatabaseAdapter) IsReady() bool {
	return true
}

func (s *ScyllaDatabaseAdapter) Insert(stats models.WikiStatsModel) error {
	return nil
}
