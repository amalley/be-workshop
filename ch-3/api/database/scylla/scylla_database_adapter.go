package scylla

import (
	"context"
	"log/slog"
	"time"

	"github.com/AMalley/be-workshop/ch-3/models"
	"github.com/gocql/gocql"
)

type ScyllaDatabaseAdapter struct {
	logger  *slog.Logger
	session *gocql.Session

	host string
}

func NewScyllaDatabaseAdapter(logger *slog.Logger, host string) *ScyllaDatabaseAdapter {
	return &ScyllaDatabaseAdapter{
		logger: logger.With(slog.String("src", "ScyllaDatabaseAdapter")),
		host:   host,
	}
}

func (s *ScyllaDatabaseAdapter) Connect(ctx context.Context) error {
	retry := time.NewTicker(5 * time.Second)
	defer retry.Stop()

	for {
		s.logger.Info("Attempting to connect to Scylla cluster...")

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-retry.C:
			cluster := gocql.NewCluster(s.host)
			cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

			cluster.Consistency = gocql.Quorum
			cluster.ConnectTimeout = 5 * time.Second
			cluster.RetryPolicy = &gocql.ExponentialBackoffRetryPolicy{
				NumRetries: 3,
			}

			session, err := cluster.CreateSession()
			if err != nil {
				s.logger.Info("Scylla connection fail, retrying...", slog.Any("err", err))
				continue
			}

			s.session = session
			s.logger.Info("Scylla connection established")
			return nil
		}
	}
}

func (s *ScyllaDatabaseAdapter) Close(ctx context.Context) error {
	return nil
}

func (s *ScyllaDatabaseAdapter) IsReady() bool {
	return s.session != nil && !s.session.Closed()
}

func (s *ScyllaDatabaseAdapter) InsertStats(stats models.WikiStatsModel) error {
	return nil
}

func (s *ScyllaDatabaseAdapter) GetStats() (models.WikiStatsCounts, error) {
	return models.WikiStatsCounts{}, nil
}
