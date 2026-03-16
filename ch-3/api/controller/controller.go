package controller

import (
	"context"
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/controller/wikistats"
)

// Controller defines the interface for the server's handler controller
type Controller interface {
	// --------------------------------------------------------------------------------------------
	// Lifecycle
	// --------------------------------------------------------------------------------------------

	OnStartup(ctx context.Context) error
	OnShutdown(ctx context.Context) error

	// --------------------------------------------------------------------------------------------
	// Health
	// --------------------------------------------------------------------------------------------

	Liveness(w http.ResponseWriter, r *http.Request)
	Readiness(w http.ResponseWriter, r *http.Request)

	// --------------------------------------------------------------------------------------------
	// User
	// --------------------------------------------------------------------------------------------

	CreateUser(w http.ResponseWriter, r *http.Request)
	DeleteUser(w http.ResponseWriter, r *http.Request)

	// --------------------------------------------------------------------------------------------
	// Stats
	// --------------------------------------------------------------------------------------------

	GetStats(w http.ResponseWriter, r *http.Request)

	// --------------------------------------------------------------------------------------------
	// Login
	// --------------------------------------------------------------------------------------------

	Login(w http.ResponseWriter, r *http.Request)
}

var _ Controller = &wikistats.WikiStatsController{}
