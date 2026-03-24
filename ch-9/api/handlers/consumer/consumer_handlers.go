package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/amalley/be-workshop/ch-9/api/authentication"
	"github.com/amalley/be-workshop/ch-9/api/database"
	"github.com/amalley/be-workshop/ch-9/api/handlers"
	"github.com/amalley/be-workshop/ch-9/api/middleware"
	"github.com/amalley/be-workshop/ch-9/api/web"
	"github.com/amalley/be-workshop/ch-9/models"
	"github.com/gocql/gocql"
	"golang.org/x/crypto/bcrypt"
)

const iss = "wikistats-app"

// Ensure ConsumerHandlers implements all required handler interfaces.
var (
	_ handlers.HealthHandlers = &ConsumerHandlers{}
	_ handlers.LoginHandlers  = &ConsumerHandlers{}
	_ handlers.StatsHandlers  = &ConsumerHandlers{}
	_ handlers.UserHandlers   = &ConsumerHandlers{}
)

type ConsumerHandlers struct {
	logger *slog.Logger

	dbAdapter     database.Adapter
	authenticator authentication.Authenticator
}

func NewConsumerHandlers(logger *slog.Logger, dbAdapter database.Adapter, authenticator authentication.Authenticator) *ConsumerHandlers {
	return &ConsumerHandlers{
		logger:        logger.With(slog.String("src", "ConsumerHandlers")),
		dbAdapter:     dbAdapter,
		authenticator: authenticator,
	}
}

func (h *ConsumerHandlers) RegisterHandlers(mux *http.ServeMux) {
	// Unauthenticated routes
	mux.Handle("GET /liveness", web.WithRequestCtx(h.logger, h.Liveness))

	mdl := middleware.NewMiddlewareRegistry()

	// Database dependent routes
	mdl.Use(h.DatabaseReadinessMiddleware(h.dbAdapter))
	mux.Handle("POST /login", mdl.Resolve(web.WithRequestCtx(h.logger, h.Login)))

	// Note: OnStartup adds an admin/password test user.
	// To create the first "real" user, login as admin. Typically, this would be done through a
	// separate admin API, but this is fine for now.

	// Authenticated routes
	mdl.Use(h.authenticator.AuthenticationMiddleware(h.verifyPublicUser))
	mux.Handle("GET /stats", mdl.Resolve(web.WithRequestCtx(h.logger, h.GetStats)))
	mux.Handle("POST /users", mdl.Resolve(web.WithRequestCtx(h.logger, h.CreateUser)))
	mux.Handle("DELETE /users", mdl.Resolve(web.WithRequestCtx(h.logger, h.DeleteUser)))
}

func (h *ConsumerHandlers) verifyPublicUser(sub string) bool {
	userID, err := gocql.ParseUUID(sub)
	if err != nil {
		h.logger.Error("Failed to parse user ID from token subject", slog.Any("err", err), slog.String("sub", sub))
		return false
	}

	_, exists, err := h.dbAdapter.GetUserByID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get user by ID", slog.Any("err", err), slog.String("user_id", sub))
		return false
	}

	if !exists {
		h.logger.Info("User not found for token subject", slog.String("user_id", sub))
		return false
	}

	return true
}

// --------------------------------------------------------------------------------------------
// Middleware
// --------------------------------------------------------------------------------------------

func (h *ConsumerHandlers) DatabaseReadinessMiddleware(database database.Adapter) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !database.IsReady() {
				h.logger.Info("Waiting for database ready")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --------------------------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------------------------

func (h *ConsumerHandlers) Liveness(ctx *web.RequestCtx) {
	ctx.Send(http.StatusOK, []byte("OK"))
}

// --------------------------------------------------------------------------------------------
// Stats
// --------------------------------------------------------------------------------------------

func (h *ConsumerHandlers) GetStats(ctx *web.RequestCtx) {
	stats, err := h.dbAdapter.GetStats(ctx.Request().Context())
	if err != nil {
		ctx.Logger().Error("Failed to get stats", slog.Any("err", err))
		ctx.SendError(http.StatusInternalServerError, fmt.Errorf("failed to get stats: %w", err))
		return
	}

	ctx.Logger().Info("Getting Stats",
		slog.Int("messages", stats.Messages),
		slog.Int("users", stats.Users),
		slog.Int("bots", stats.Bots),
		slog.Int("servers", stats.Servers),
	)

	ctx.SendJSON(http.StatusOK, stats)
}

// --------------------------------------------------------------------------------------------
// User
// --------------------------------------------------------------------------------------------

func (h *ConsumerHandlers) CreateUser(ctx *web.RequestCtx) {
	if ctx.Request().ContentLength == 0 {
		ctx.SendError(http.StatusBadRequest, fmt.Errorf("no request body provided"))
		return
	}

	var newUser models.CreateUserRequest

	if err := json.NewDecoder(ctx.Request().Body).Decode(&newUser); err != nil {
		ctx.SendError(http.StatusBadRequest, fmt.Errorf("invalid or missing JSON body: %w", err))
		return
	}

	_, exists, err := h.dbAdapter.GetUser(ctx.Request().Context(), newUser.Username)
	if err != nil {
		ctx.Logger().Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		ctx.SendError(http.StatusInternalServerError, fmt.Errorf("internal server error: %w", err))
		return
	}

	if exists {
		ctx.Logger().Error("Failed to create user", slog.String("err", "user already exists"), slog.String("user", newUser.Username))
		ctx.SendError(http.StatusConflict, fmt.Errorf("user '%s' already exists", newUser.Username))
		return
	}

	if err := h.dbAdapter.CreateUser(ctx.Request().Context(), newUser.Username, newUser.Password); err != nil {
		ctx.Logger().Error("Failed to create user", slog.Any("err", err), slog.String("user", newUser.Username))
		ctx.SendError(http.StatusInternalServerError, fmt.Errorf("failed to create user: %w", err))
		return
	}

	ctx.Logger().Info("User created", slog.String("user", newUser.Username))
	ctx.Send(http.StatusOK, []byte("OK"))
}

func (h *ConsumerHandlers) DeleteUser(ctx *web.RequestCtx) {
	username := ctx.Request().URL.Query().Get("user")
	if username == "" {
		ctx.SendError(http.StatusBadRequest, fmt.Errorf("no user query parameter provided"))
		return
	}

	userDB, exists, err := h.dbAdapter.GetUser(ctx.Request().Context(), username)
	if err != nil {
		ctx.Logger().Error("Failed to get user", slog.Any("err", err), slog.String("user", username))
		ctx.SendError(http.StatusInternalServerError, fmt.Errorf("failed to get user: %w", err))
		return
	}

	if !exists {
		ctx.Logger().Error("Failed to delete user", slog.String("err", "user not found"), slog.String("user", username))
		ctx.SendError(http.StatusNotFound, fmt.Errorf("user '%s' not found", username))
		return
	}

	if err := h.dbAdapter.DeleteUser(ctx.Request().Context(), userDB.ID); err != nil {
		ctx.Logger().Error("Failed to delete user", slog.Any("err", err), slog.String("user", username), slog.String("user_id", userDB.ID.String()))
		ctx.SendError(http.StatusInternalServerError, fmt.Errorf("failed to delete user: %w", err))
		return
	}

	ctx.Logger().Info("User deleted", slog.String("user", username), slog.String("user_id", userDB.ID.String()))
	ctx.Send(http.StatusOK, []byte("OK"))
}

// --------------------------------------------------------------------------------------------
// Login
// --------------------------------------------------------------------------------------------

func (h *ConsumerHandlers) Login(ctx *web.RequestCtx) {
	if ctx.Request().ContentLength == 0 {
		ctx.SendError(http.StatusBadRequest, fmt.Errorf("no request body provided"))
		return
	}

	var login models.LoginRequest

	if err := json.NewDecoder(ctx.Request().Body).Decode(&login); err != nil {
		ctx.SendError(http.StatusBadRequest, fmt.Errorf("invalid or missing JSON body: %w", err))
		return
	}

	userDB, exists, err := h.dbAdapter.GetUser(ctx.Request().Context(), login.Username)
	if err != nil {
		ctx.Logger().Error("Failed to login", slog.Any("err", err), slog.String("user", login.Username))
		ctx.SendError(http.StatusInternalServerError, fmt.Errorf("internal server error: %w", err))
		return
	}

	if !exists {
		ctx.Logger().Error("Failed to login", slog.String("err", "user not found"), slog.String("user", login.Username))
		ctx.SendError(http.StatusUnauthorized, fmt.Errorf("user '%s' not found", login.Username))
		return
	}

	if err := bcrypt.CompareHashAndPassword(userDB.Password, []byte(login.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			ctx.Logger().Error("Failed to login", slog.String("err", "Invalid credentials"), slog.String("user", login.Username))
			ctx.SendError(http.StatusUnauthorized, fmt.Errorf("invalid credentials"))
			return
		}
		ctx.Logger().Error("Failed to login", slog.Any("err", err), slog.String("user", login.Username))
		ctx.SendError(http.StatusUnauthorized, fmt.Errorf("failed to login: %w", err))
		return
	}

	token, err := h.authenticator.GenerateToken(iss, userDB.ID.String())
	if err != nil {
		ctx.Logger().Error("Failed to login", slog.Any("err", err), slog.String("user", login.Username))
		ctx.SendError(http.StatusUnauthorized, fmt.Errorf("failed to login: %w", err))
		return
	}

	ctx.Logger().Info("Login successful", slog.String("user", login.Username))
	ctx.SendJSON(http.StatusOK, models.LoginResponse{
		Token: token,
	})
}
