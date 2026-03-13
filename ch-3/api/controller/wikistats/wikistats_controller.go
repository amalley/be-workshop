package wikistats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/models"
)

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

func (c *WikiStatsController) Liveness(w http.ResponseWriter, r *http.Request) {
	c.logger.Info("We're alive!")
	w.WriteHeader(http.StatusOK)
}

func (c *WikiStatsController) Readiness(w http.ResponseWriter, r *http.Request) {
	if !c.database.IsReady() {
		c.logger.Info("Waiting for database ready")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	c.logger.Info("We're ready!")
	w.WriteHeader(http.StatusOK)
}

// --------------------------------------------------------------------------------------------
// Stats
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) GetStats(w http.ResponseWriter, r *http.Request) {
	messages, users, bots, servers := 0, 0, 0, 0

	c.logger.Info("Getting Stats",
		slog.Int("messages", messages),
		slog.Int("users", users),
		slog.Int("bots", bots),
		slog.Int("servers", servers),
	)

	w.WriteHeader(http.StatusOK)
}

// --------------------------------------------------------------------------------------------
// User
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength == 0 {
		http.Error(w, "No request body provided", http.StatusBadRequest)
		return
	}

	var newUser models.User

	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Invalid or missing JSON body", http.StatusBadRequest)
		return
	}

	_, exists, err := c.database.GetUser(r.Context(), newUser.Username)
	if err != nil {
		c.logger.Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		http.Error(w, fmt.Sprintf("Internal server error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if exists {
		c.logger.Error("Failed to create user", slog.String("err", "user already exists"), slog.String("user", newUser.Username))
		http.Error(w, fmt.Sprintf("User '%s' already exists", newUser.Username), http.StatusConflict)
		return
	}

	if err := c.database.CreateUser(r.Context(), newUser.Username, newUser.Password); err != nil {
		c.logger.Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		http.Error(w, fmt.Sprintf("Failed to create user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	c.logger.Info("User created", slog.String("user", newUser.Username))
	w.WriteHeader(http.StatusCreated)
}

func (c *WikiStatsController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user")

	if username == "" {
		http.Error(w, "No user query parameter provided", http.StatusBadRequest)
		return
	}

	if err := c.database.DeleteUser(r.Context(), username); err != nil {
		c.logger.Error("Failed to delete user", slog.Any("err", err), slog.String("user", username))
		http.Error(w, fmt.Sprintf("Failed to delete user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	c.logger.Info("User deleted", slog.String("user", username))
	w.WriteHeader(http.StatusOK)
}
