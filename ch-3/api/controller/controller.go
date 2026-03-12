package controller

import (
	"log/slog"
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/database"
)

type Controller struct {
	logger   *slog.Logger
	database *database.WikiStatsDB
}

func NewController(logger *slog.Logger, database *database.WikiStatsDB) *Controller {
	return &Controller{
		logger:   logger,
		database: database,
	}
}

func (c *Controller) Liveness(writer http.ResponseWriter, req *http.Request) {
	c.logger.Info("We're alive!")
	writer.WriteHeader(http.StatusOK)
}

func (c *Controller) GetStats(writer http.ResponseWriter, req *http.Request) {
	messages, users, bots, servers := c.database.GetCounts()

	c.logger.Info("Getting Stats",
		slog.Int("messages", messages),
		slog.Int("users", users),
		slog.Int("bots", bots),
		slog.Int("servers", servers),
	)

	writer.WriteHeader(http.StatusOK)
}
