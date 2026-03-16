package wikistats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/AMalley/be-workshop/ch-3/api/authentication"
	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
	"github.com/AMalley/be-workshop/ch-3/api/stream"
	"github.com/AMalley/be-workshop/ch-3/api/utils"
	"github.com/AMalley/be-workshop/ch-3/models"
	"github.com/gocql/gocql"
	"golang.org/x/crypto/bcrypt"
)

type WikiStatsController struct {
	logger *slog.Logger

	stream        stream.StreamAdapter
	database      database.DatabaseAdpater
	authenticator authentication.Authenticator
}

func NewWikiStatsController(
	logger *slog.Logger,
	stream stream.StreamAdapter,
	database database.DatabaseAdpater,
	authenticator authentication.Authenticator) *WikiStatsController {
	return &WikiStatsController{
		logger:        logger.With(slog.String("src", "WikiStatsController")),
		stream:        stream,
		database:      database,
		authenticator: authenticator,
	}
}

func (c *WikiStatsController) RegisterRoutes(mux *http.ServeMux) {
	// Unauthenticated routes
	mux.Handle("GET /health/liveness", http.HandlerFunc(c.Liveness))
	mux.Handle("GET /health/readiness", http.HandlerFunc(c.Readiness))
	mux.Handle("POST /login", http.HandlerFunc(c.Login))

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(c.DatabaseReadinessMiddleware(c.database))
	mdl.Use(c.authenticator.AuthenticationMiddleware(c.VerifyPublicUser))

	// Note: OnStartup adds an admin/password test user.
	// To create first "real" user, login as admin. Typically, this would be done through a separate admin API, but now this is fine.

	// Authenticated routes
	mux.Handle("GET /stats", mdl.Resolve(http.HandlerFunc(c.GetStats)))
	mux.Handle("POST /users", mdl.Resolve(http.HandlerFunc(c.CreateUser)))
	mux.Handle("DELETE /users", mdl.Resolve(http.HandlerFunc(c.DeleteUser)))
}

func (c *WikiStatsController) GetCtxLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger.With(slog.String("src", "WikiStatsController"))
	}
	return c.logger
}

func (c *WikiStatsController) VerifyPublicUser(sub string) bool {
	_, exists, err := c.database.GetUserByID(context.Background(), sub)
	if err != nil {
		c.logger.Error("Failed to get user by ID", slog.Any("err", err), slog.String("user_id", sub))
		return false
	}

	_, err = gocql.ParseUUID(sub)
	if err != nil {
		c.logger.Error("Failed to parse user ID from token subject", slog.Any("err", err), slog.String("sub", sub))
		return false
	}

	if !exists {
		c.logger.Info("User not found for token subject", slog.String("user_id", sub))
		return false
	}

	return true
}

// --------------------------------------------------------------------------------------------
// Middleware
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) DatabaseReadinessMiddleware(database database.DatabaseAdpater) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !database.IsReady() {
				c.GetCtxLogger(r.Context()).Info("Waiting for database ready")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --------------------------------------------------------------------------------------------
// Lifecycle
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) OnStartup(ctx context.Context) error {
	if utils.CtxDone(ctx) {
		c.logger.Info("failed to start stream", slog.String("reason", ctx.Err().Error()))
		return ctx.Err()
	}

	var grp sync.WaitGroup
	grp.Go(func() {
		if err := c.stream.Connect(ctx); err != nil {
			c.logger.Error("Failed to connect to stream", slog.Any("err", err))
		}
	})
	grp.Go(func() {
		if err := c.database.Connect(ctx); err != nil {
			c.logger.Error("Failed to connect to database", slog.Any("err", err))
		}
	})
	grp.Wait()

	// We'll give the database a few seconds to be ready.
	wait := time.NewTicker(time.Second * 10)
	defer wait.Stop()

	for !c.database.IsReady() {
		c.logger.Info("Waiting for database to be ready...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait.C:
		}
	}

	if c.database.IsReady() {
		_, exists, err := c.database.GetUser(ctx, "admin")
		if err != nil {
			c.logger.Error("Failed to get default user", slog.Any("err", err))
		}
		if !exists {
			// Don't do this in production
			if err := c.database.CreateUser(ctx, "admin", "password"); err != nil {
				c.logger.Error("Failed to create default user", slog.Any("err", err))
			}
		}
	}

	go func() {
		c.logger.Info("Starting stream consumption")
		if err := c.stream.Consume(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error("Failed to consume stream", slog.Any("err", err))
		}
	}()

	return nil
}

func (c *WikiStatsController) OnShutdown(ctx context.Context) error {
	if utils.CtxDone(ctx) {
		c.logger.Info("failed to stop stream", slog.String("reason", ctx.Err().Error()))
		return ctx.Err()
	}

	if err := c.stream.Close(ctx); err != nil {
		c.logger.Error("Failed to close stream", slog.Any("err", err))
	}

	if err := c.database.Close(ctx); err != nil {
		c.logger.Error("Failed to close database", slog.Any("err", err))
	}

	return nil
}

// --------------------------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) Liveness(w http.ResponseWriter, r *http.Request) {
	c.GetCtxLogger(r.Context()).Info("We're alive!")
	w.WriteHeader(http.StatusOK)
}

func (c *WikiStatsController) Readiness(w http.ResponseWriter, r *http.Request) {
	logger := c.GetCtxLogger(r.Context())

	if !c.database.IsReady() {
		logger.Info("Waiting for database ready")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	logger.Info("We're ready!")
	w.WriteHeader(http.StatusOK)
}

// --------------------------------------------------------------------------------------------
// Stats
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := c.database.GetStats(r.Context())
	logger := c.GetCtxLogger(r.Context())

	if err != nil {
		logger.Error("Failed to get stats", slog.Any("err", err))
		http.Error(w, fmt.Sprintf("Failed to get stats: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	logger.Info("Getting Stats",
		slog.Int("messages", stats.Messages),
		slog.Int("users", stats.Users),
		slog.Int("bots", stats.Bots),
		slog.Int("servers", stats.Servers),
	)

	w.WriteHeader(http.StatusOK)
}

// --------------------------------------------------------------------------------------------
// User
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) CreateUser(w http.ResponseWriter, r *http.Request) {
	logger := c.GetCtxLogger(r.Context())

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
		logger.Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		http.Error(w, fmt.Sprintf("Internal server error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if exists {
		logger.Error("Failed to create user", slog.String("err", "user already exists"), slog.String("user", newUser.Username))
		http.Error(w, fmt.Sprintf("User '%s' already exists", newUser.Username), http.StatusConflict)
		return
	}

	if err := c.database.CreateUser(r.Context(), newUser.Username, newUser.Password); err != nil {
		logger.Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		http.Error(w, fmt.Sprintf("Failed to create user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	logger.Info("User created", slog.String("user", newUser.Username))
	w.WriteHeader(http.StatusCreated)
}

func (c *WikiStatsController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	logger := c.GetCtxLogger(r.Context())
	username := r.URL.Query().Get("user")

	if username == "" {
		http.Error(w, "No user query parameter provided", http.StatusBadRequest)
		return
	}

	if err := c.database.DeleteUser(r.Context(), username); err != nil {
		logger.Error("Failed to delete user", slog.Any("err", err), slog.String("user", username))
		http.Error(w, fmt.Sprintf("Failed to delete user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	logger.Info("User deleted", slog.String("user", username))
	w.WriteHeader(http.StatusOK)
}

// --------------------------------------------------------------------------------------------
// Login
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) Login(w http.ResponseWriter, r *http.Request) {
	logger := c.GetCtxLogger(r.Context())

	if r.ContentLength == 0 {
		http.Error(w, "No request body provided", http.StatusBadRequest)
		return
	}

	var user models.User

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid or missing JSON body", http.StatusBadRequest)
		return
	}

	userDB, exists, err := c.database.GetUser(r.Context(), user.Username)
	if err != nil {
		logger.Error("Failed to login", slog.Any("err", err), slog.String("user", user.Username))
		http.Error(w, fmt.Sprintf("Internal server error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if !exists {
		logger.Error("Failed to login", slog.String("err", "user not found"), slog.String("user", user.Username))
		http.Error(w, fmt.Sprintf("User '%s' not found", user.Username), http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword(userDB.Password, []byte(user.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			logger.Error("Failed to login", slog.String("err", "Invalid credentials"), slog.String("user", user.Username))
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		logger.Error("Failed to login", slog.Any("err", err), slog.String("user", user.Username))
		http.Error(w, "Failed to login", http.StatusUnauthorized)
		return
	}

	token, err := c.authenticator.GenerateToken(userDB.ID)
	if err != nil {
		logger.Error("Failed to login", slog.Any("err", err), slog.String("user", user.Username))
		http.Error(w, "Failed to login", http.StatusUnauthorized)
		return
	}

	logger.Info("Login successful", slog.String("user", user.Username))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(models.LoginResponse{
		Token: token,
	})
}
