package scylla

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/models"
	"github.com/gocql/gocql"
	"golang.org/x/crypto/bcrypt"
)

const (
	KeyspaceQuery = `CREATE KEYSPACE IF NOT EXISTS wikistats WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`

	UsersTable = `CREATE TABLE IF NOT EXISTS wikistats.users (
        user_id uuid,
        username text,
        password text,
        created_on timestamp,
        PRIMARY KEY (user_id)
    )`

	UsernameIndex = `CREATE MATERIALIZED VIEW IF NOT EXISTS wikistats.users_by_username AS
		SELECT * FROM wikistats.users
		WHERE username IS NOT NULL AND user_id IS NOT NULL
		PRIMARY KEY (username, user_id);`

	StatsTable = `CREATE TABLE IF NOT EXISTS wikistats.stats (
		stat_type text,
		time_bucket timestamp,
		stat_value counter,
		PRIMARY KEY (stat_type, time_bucket)
	) WITH CLUSTERING ORDER BY (time_bucket DESC);`
)

const (
	MessagesStat = "messages"
	UsersStat    = "users"
	BotsStat     = "bots"
	ServersStat  = "servers"
)

var _ database.DatabaseAdapter = &ScyllaDatabaseAdapter{}

type ScyllaDatabaseAdapter struct {
	logger  *slog.Logger
	session *gocql.Session
	cfg     *ScyllaOptions
}

func NewScyllaDatabaseAdapter(logger *slog.Logger, opts ...ScyllaOption) *ScyllaDatabaseAdapter {
	cfg := applyOptions(defaultOptions(), opts...)
	return &ScyllaDatabaseAdapter{
		logger: logger.With(slog.String("src", "ScyllaDatabaseAdapter")),
		cfg:    cfg,
	}
}

func (s *ScyllaDatabaseAdapter) Connect(ctx context.Context) error {
	if s.cfg.Host == "" {
		return errors.New("No Scylla host provided")
	}
	if s.cfg.Keyspace == "" {
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

	s.session.Close()
	s.session = nil

	return nil
}

func (s *ScyllaDatabaseAdapter) IsReady() bool {
	return s.session != nil && !s.session.Closed()
}

func (s *ScyllaDatabaseAdapter) tryConnent(ctx context.Context) error {
	retry := time.NewTicker(s.cfg.RetryTime)
	defer retry.Stop()

	for {
		s.logger.Info("Attempting to connect to Scylla cluster...")

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-retry.C:
			cluster := gocql.NewCluster(s.cfg.Host)

			cluster.PoolConfig.HostSelectionPolicy = s.cfg.HostSelectPolicy
			cluster.DisableInitialHostLookup = s.cfg.DisableInitialHostLookup
			cluster.Consistency = s.cfg.ClusterConsistency
			cluster.ConnectTimeout = s.cfg.ConnectionTimeout

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
	queries := []string{
		KeyspaceQuery,
		UsersTable,
		UsernameIndex,
		StatsTable,
	}

	for _, query := range queries {
		if err := s.session.Query(query).WithContext(ctx).Exec(); err != nil {
			return err
		}
	}

	s.logger.Info("Scylla keyspace and tables ensured")
	return nil
}

// --------------------------------------------------------------------------------------------
// Stats
// --------------------------------------------------------------------------------------------

func (s *ScyllaDatabaseAdapter) InsertStats(ctx context.Context, stats models.WikiStatsModel) error {
	const query = `UPDATE wikistats.stats SET stat_value = stat_value + 1 WHERE stat_type = ? AND time_bucket = ?`

	now := time.Now().UTC().Truncate(time.Minute)

	if err := s.session.Query(query, MessagesStat, now).WithContext(ctx).Exec(); err != nil {
		return err
	}

	label := UsersStat
	if stats.IsBot {
		label = BotsStat
	}

	if err := s.session.Query(query, label, now).WithContext(ctx).Exec(); err != nil {
		return err
	}

	return s.session.Query(query, ServersStat, now).WithContext(ctx).Exec()
}

func (s *ScyllaDatabaseAdapter) GetStats(ctx context.Context) (models.WikiStatsCounts, error) {
	const query = `SELECT stat_type, SUM(stat_value) FROM wikistats.stats GROUP BY stat_type`

	var stats models.WikiStatsCounts
	var statType string
	var total int64

	iter := s.session.Query(query).WithContext(ctx).Iter()
	for iter.Scan(&statType, &total) {
		val := int(total)
		switch statType {
		case MessagesStat:
			stats.Messages = val
		case UsersStat:
			stats.Users = val
		case BotsStat:
			stats.Bots = val
		case ServersStat:
			stats.Servers = val
		}
	}

	return stats, iter.Close()
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

	uid := gocql.TimeUUID()
	return s.session.Query(query, uid, username, hashedPassword, time.Now().UTC()).WithContext(ctx).Exec()
}

func (s *ScyllaDatabaseAdapter) DeleteUser(ctx context.Context, userID gocql.UUID) error {
	const query = `DELETE FROM wikistats.users WHERE user_id = ?`
	return s.session.Query(query, userID).WithContext(ctx).Exec()
}

func (s *ScyllaDatabaseAdapter) GetUser(ctx context.Context, username string) (models.User, bool, error) {
	const query = `SELECT user_id, username, password, created_on FROM wikistats.users_by_username WHERE username = ?`

	var user models.User

	err := s.session.Query(query, username).WithContext(ctx).Consistency(gocql.One).
		Scan(&user.ID, &user.Username, &user.Password, &user.CreatedOn)

	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return user, false, nil
		}
		return user, false, err
	}

	return user, true, nil
}

func (s *ScyllaDatabaseAdapter) GetUserByID(ctx context.Context, userID gocql.UUID) (models.User, bool, error) {
	const query = `SELECT user_id, username, password, created_on FROM wikistats.users WHERE user_id = ?`

	var user models.User

	err := s.session.Query(query, userID).WithContext(ctx).Consistency(gocql.One).
		Scan(&user.ID, &user.Username, &user.Password, &user.CreatedOn)

	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return user, false, nil
		}
		return user, false, err
	}

	return user, true, nil
}
