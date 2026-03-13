package wikistats

import (
	"log/slog"
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/database"
)

type DatabaseAdapter interface {
	GetStats() (int, int, int, int)
	ReadyCheck() bool
}

type WikiStatsController struct {
	logger *slog.Logger

	database database.DatabaseAdpater
}

func NewWikiStatsController(logger *slog.Logger, database database.DatabaseAdpater) *WikiStatsController {
	return &WikiStatsController{
		logger:   logger.With(slog.String("src", "WikiStatsController")),
		database: database,
	}
}

func (c *WikiStatsController) Liveness(writer http.ResponseWriter, req *http.Request) {
	c.logger.Info("We're alive!")
	writer.WriteHeader(http.StatusOK)
}

func (c *WikiStatsController) Readiness(writer http.ResponseWriter, req *http.Request) {
	if !c.database.IsReady() {
		c.logger.Info("Waiting for database ready")
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	c.logger.Info("We're ready!")
	writer.WriteHeader(http.StatusOK)
}

func (c *WikiStatsController) GetStats(writer http.ResponseWriter, req *http.Request) {
	messages, users, bots, servers := 0, 0, 0, 0

	c.logger.Info("Getting Stats",
		slog.Int("messages", messages),
		slog.Int("users", users),
		slog.Int("bots", bots),
		slog.Int("servers", servers),
	)

	writer.WriteHeader(http.StatusOK)
}
