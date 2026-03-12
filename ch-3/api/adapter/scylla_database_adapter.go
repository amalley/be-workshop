package adapter

import (
	"context"
	"log/slog"
)

type ScyllaDatabaseAdapter struct {
	logger *slog.Logger
	host   string
}

func NewScyllaDatabaseAdapter(logger *slog.Logger, host string) *ScyllaDatabaseAdapter {
	return &ScyllaDatabaseAdapter{
		logger: logger,
		host:   host,
	}
}

func (s *ScyllaDatabaseAdapter) Connect(ctx context.Context) error {
	return nil
}

func (s *ScyllaDatabaseAdapter) Close(ctx context.Context) error {
	return nil
}
