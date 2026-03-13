package scylla

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/AMalley/be-workshop/ch-3/models"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	UsersTable = `CREATE TABLE IF NOT EXISTS wikistats.users (
        user_id uuid,
        username text,
        password text,
        created_on timestamp,
        PRIMARY KEY (username)
    )`

	StatsTable = `CREATE TABLE IF NOT EXISTS wikistats.global_counts (
		stat_type text PRIMARY KEY,
		stat_value counter
	);`
)

type ScyllaDatabaseAdapter struct {
	logger  *slog.Logger
	session *gocql.Session

	host     string
	keyspace string
}

func NewScyllaDatabaseAdapter(logger *slog.Logger, host, keyspace string) *ScyllaDatabaseAdapter {
	return &ScyllaDatabaseAdapter{
		logger:   logger.With(slog.String("src", "ScyllaDatabaseAdapter")),
		host:     host,
		keyspace: keyspace,
	}
}

func (s *ScyllaDatabaseAdapter) Connect(ctx context.Context) error {
	if s.host == "" {
		return errors.New("No Scylla host provided")
	}
	if s.keyspace == "" {
		return errors.New("No Scylla keyspace provided")
	}

	if err := s.tryConnent(ctx); err != nil {
		return err
	}

	if err := s.tryCreateKeyspace(ctx); err != nil {
		return err
	}

	return nil
}

func (s *ScyllaDatabaseAdapter) Close(ctx context.Context) error {
	if s.session == nil {
		return nil
	}
	return s.Close(ctx)
}

func (s *ScyllaDatabaseAdapter) IsReady() bool {
	return s.session != nil && !s.session.Closed()
}

func (s *ScyllaDatabaseAdapter) tryConnent(ctx context.Context) error {
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

func (s *ScyllaDatabaseAdapter) tryCreateKeyspace(ctx context.Context) error {
	const ksquery = `CREATE KEYSPACE IF NOT EXISTS wikistats WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`

	if err := s.session.Query(ksquery).WithContext(ctx).Exec(); err != nil {
		return nil
	}

	if err := s.session.Query(UsersTable).WithContext(ctx).Exec(); err != nil {
		return nil
	}

	if err := s.session.Query(StatsTable).WithContext(ctx).Exec(); err != nil {
		return nil
	}

	return nil
}

// --------------------------------------------------------------------------------------------
// Stats
// --------------------------------------------------------------------------------------------

func (s *ScyllaDatabaseAdapter) InsertStats(ctx context.Context, stats models.WikiStatsModel) error {
	return nil
}

func (s *ScyllaDatabaseAdapter) GetStats(ctx context.Context) (models.WikiStatsCounts, error) {
	return models.WikiStatsCounts{}, nil
}

// --------------------------------------------------------------------------------------------
// User
// --------------------------------------------------------------------------------------------

func (s *ScyllaDatabaseAdapter) CreateUser(ctx context.Context, username, password string) error {
	const query = `INSERT INTO wikistats.users (user_id, username, password, created_on) VALUES (?, ?, ?, ?)`

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return err
	}

	return s.session.Query(query, uuid.New().String(), username, hashedPassword, time.Now().UTC()).WithContext(ctx).Exec()
}

func (s *ScyllaDatabaseAdapter) DeleteUser(ctx context.Context, username string) error {
	const query = `DELETE FROM wikistats.users WHERE username = ?`
	return s.session.Query(query, username).WithContext(ctx).Exec()
}

func (s *ScyllaDatabaseAdapter) GetUser(ctx context.Context, username string) (models.User_DB, bool, error) {
	const query = `SELECT user_id, username, password, created_on FROM wikistats.users WHERE username = ?`

	var user models.User_DB
	var idStr string

	err := s.session.Query(query, username).WithContext(ctx).Consistency(gocql.One).
		Scan(&idStr, &user.Username, &user.Password, &user.CreatedOn)

	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return user, false, nil
		}
		return user, false, err
	}

	user.ID, _ = uuid.Parse(idStr)
	return user, true, nil
}
