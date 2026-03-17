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
	"github.com/AMalley/be-workshop/ch-3/api/controller"
	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
	"github.com/AMalley/be-workshop/ch-3/api/stream"
	"github.com/AMalley/be-workshop/ch-3/api/utils"
	"github.com/AMalley/be-workshop/ch-3/models"
	"github.com/gocql/gocql"
	"golang.org/x/crypto/bcrypt"
)

var _ controller.Controller = &WikiStatsController{}

type WikiStatsController struct {
	logger *slog.Logger

	stream        stream.StreamAdapter
	database      database.DatabaseAdapter
	authenticator authentication.Authenticator
}

func NewWikiStatsController(
	logger *slog.Logger,
	stream stream.StreamAdapter,
	database database.DatabaseAdapter,
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
	mux.Handle("GET /health/liveness", c.handler(c.Liveness))

	mdl := middleware.NewMiddlewareRegistry()

	// Database dependent routes
	mdl.Use(c.DatabaseReadinessMiddleware(c.database))
	mux.Handle("POST /login", mdl.Resolve(c.handler(c.Login)))

	// Note: OnStartup adds an admin/password test user.
	// To create the first "real" user, login as admin. Typically, this would be done through a separate admin API, but this is fine for now.

	// Authenticated routes
	mdl.Use(c.authenticator.AuthenticationMiddleware(c.VerifyPublicUser))
	mux.Handle("GET /stats", mdl.Resolve(c.handler(c.GetStats)))
	mux.Handle("POST /users", mdl.Resolve(c.handler(c.CreateUser)))
	mux.Handle("DELETE /users", mdl.Resolve(c.handler(c.DeleteUser)))
}

func (c *WikiStatsController) VerifyPublicUser(sub string) bool {
	userID, err := gocql.ParseUUID(sub)
	if err != nil {
		c.logger.Error("Failed to parse user ID from token subject", slog.Any("err", err), slog.String("sub", sub))
		return false
	}

	_, exists, err := c.database.GetUserByID(context.Background(), userID)
	if err != nil {
		c.logger.Error("Failed to get user by ID", slog.Any("err", err), slog.String("user_id", sub))
		return false
	}

	if !exists {
		c.logger.Info("User not found for token subject", slog.String("user_id", sub))
		return false
	}

	return true
}

// handler wraps a models.RequestCtx around the standard http.HandlerFunc, allowing us to use our custom context with logging and other utilities in our handlers.
func (c *WikiStatsController) handler(handler func(*models.RequestCtx)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(models.NewRequestCtx(c.logger, w, r))
	}
}

// --------------------------------------------------------------------------------------------
// Middleware
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) DatabaseReadinessMiddleware(database database.DatabaseAdapter) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !database.IsReady() {
				c.logger.Info("Waiting for database ready")
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

	wait := time.NewTicker(time.Second * 2)
	defer wait.Stop()

	// Ensure the database infrastructure is ready before we start consuming the stream
	for !c.database.IsReady() {
		c.logger.Info("Waiting for database to be ready...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait.C:
		}
	}

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
		c.logger.Info("failed to shutdown", slog.String("reason", ctx.Err().Error()))
		return ctx.Err()
	}

	// Note: We don't return errors here to avoid interrupting shutdown, but we do log them.

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

func (c *WikiStatsController) Liveness(ctx *models.RequestCtx) {
	ctx.Logger().Info("We're alive!")
	ctx.Response().WriteHeader(http.StatusOK)
	ctx.Response().Write([]byte("OK"))
}

// --------------------------------------------------------------------------------------------
// Stats
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) GetStats(ctx *models.RequestCtx) {
	stats, err := c.database.GetStats(ctx.Request().Context())

	if err != nil {
		ctx.Logger().Error("Failed to get stats", slog.Any("err", err))
		http.Error(ctx.Response(), fmt.Sprintf("Failed to get stats: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	ctx.Logger().Info("Getting Stats",
		slog.Int("messages", stats.Messages),
		slog.Int("users", stats.Users),
		slog.Int("bots", stats.Bots),
		slog.Int("servers", stats.Servers),
	)

	ctx.Response().Header().Set("Content-Type", "application/json")
	ctx.Response().WriteHeader(http.StatusOK)

	json.NewEncoder(ctx.Response()).Encode(models.GetStatsResponse{
		Messages: stats.Messages,
		Users:    stats.Users,
		Bots:     stats.Bots,
		Servers:  stats.Servers,
	})
}

// --------------------------------------------------------------------------------------------
// User
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) CreateUser(ctx *models.RequestCtx) {
	if ctx.Request().ContentLength == 0 {
		http.Error(ctx.Response(), "No request body provided", http.StatusBadRequest)
		return
	}

	var newUser models.CreateUserRequest

	if err := json.NewDecoder(ctx.Request().Body).Decode(&newUser); err != nil {
		http.Error(ctx.Response(), "Invalid or missing JSON body", http.StatusBadRequest)
		return
	}

	_, exists, err := c.database.GetUser(ctx.Request().Context(), newUser.Username)
	if err != nil {
		ctx.Logger().Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		http.Error(ctx.Response(), fmt.Sprintf("Internal server error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if exists {
		ctx.Logger().Error("Failed to create user", slog.String("err", "user already exists"), slog.String("user", newUser.Username))
		http.Error(ctx.Response(), fmt.Sprintf("User '%s' already exists", newUser.Username), http.StatusConflict)
		return
	}

	if err := c.database.CreateUser(ctx.Request().Context(), newUser.Username, newUser.Password); err != nil {
		ctx.Logger().Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		http.Error(ctx.Response(), fmt.Sprintf("Failed to create user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	ctx.Logger().Info("User created", slog.String("user", newUser.Username))

	ctx.Response().WriteHeader(http.StatusCreated)
	ctx.Response().Write([]byte("OK"))
}

func (c *WikiStatsController) DeleteUser(ctx *models.RequestCtx) {
	username := ctx.Request().URL.Query().Get("user")

	if username == "" {
		http.Error(ctx.Response(), "No user query parameter provided", http.StatusBadRequest)
		return
	}

	userDB, exists, err := c.database.GetUser(ctx.Request().Context(), username)
	if err != nil {
		ctx.Logger().Error("Failed to get user", slog.Any("err", err), slog.String("user", username))
		http.Error(ctx.Response(), fmt.Sprintf("Failed to get user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if !exists {
		ctx.Logger().Error("Failed to delete user", slog.String("err", "user not found"), slog.String("user", username))
		http.Error(ctx.Response(), fmt.Sprintf("User '%s' not found", username), http.StatusNotFound)
		return
	}

	if err := c.database.DeleteUser(ctx.Request().Context(), userDB.ID); err != nil {
		ctx.Logger().Error("Failed to delete user", slog.Any("err", err), slog.String("user", username), slog.String("user_id", userDB.ID.String()))
		http.Error(ctx.Response(), fmt.Sprintf("Failed to delete user: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	ctx.Logger().Info("User deleted", slog.String("user", username), slog.String("user_id", userDB.ID.String()))

	ctx.Response().WriteHeader(http.StatusOK)
	ctx.Response().Write([]byte("OK"))
}

// --------------------------------------------------------------------------------------------
// Login
// --------------------------------------------------------------------------------------------

func (c *WikiStatsController) Login(ctx *models.RequestCtx) {
	logger := ctx.Logger()

	if ctx.Request().ContentLength == 0 {
		http.Error(ctx.Response(), "No request body provided", http.StatusBadRequest)
		return
	}

	var login models.LoginRequest

	if err := json.NewDecoder(ctx.Request().Body).Decode(&login); err != nil {
		http.Error(ctx.Response(), "Invalid or missing JSON body", http.StatusBadRequest)
		return
	}

	userDB, exists, err := c.database.GetUser(ctx.Request().Context(), login.Username)
	if err != nil {
		logger.Error("Failed to login", slog.Any("err", err), slog.String("user", login.Username))
		http.Error(ctx.Response(), fmt.Sprintf("Internal server error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if !exists {
		logger.Error("Failed to login", slog.String("err", "user not found"), slog.String("user", login.Username))
		http.Error(ctx.Response(), fmt.Sprintf("User '%s' not found", login.Username), http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword(userDB.Password, []byte(login.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			logger.Error("Failed to login", slog.String("err", "Invalid credentials"), slog.String("user", login.Username))
			http.Error(ctx.Response(), "Invalid credentials", http.StatusUnauthorized)
			return
		}
		logger.Error("Failed to login", slog.Any("err", err), slog.String("user", login.Username))
		http.Error(ctx.Response(), "Failed to login", http.StatusUnauthorized)
		return
	}

	token, err := c.authenticator.GenerateToken(userDB.ID.String())
	if err != nil {
		logger.Error("Failed to login", slog.Any("err", err), slog.String("user", login.Username))
		http.Error(ctx.Response(), "Failed to login", http.StatusUnauthorized)
		return
	}

	logger.Info("Login successful", slog.String("user", login.Username))

	ctx.Response().Header().Set("Content-Type", "application/json")
	ctx.Response().WriteHeader(http.StatusOK)

	json.NewEncoder(ctx.Response()).Encode(models.LoginResponse{
		Token: token,
	})
}
