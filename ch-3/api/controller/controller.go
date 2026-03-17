package controller

import (
	"context"

	"github.com/AMalley/be-workshop/ch-3/api/controller/wikistats"
	"github.com/AMalley/be-workshop/ch-3/models"
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

	Liveness(*models.RequestCtx)

	// --------------------------------------------------------------------------------------------
	// User
	// --------------------------------------------------------------------------------------------

	CreateUser(*models.RequestCtx)
	DeleteUser(*models.RequestCtx)

	// --------------------------------------------------------------------------------------------
	// Stats
	// --------------------------------------------------------------------------------------------

	GetStats(*models.RequestCtx)

	// --------------------------------------------------------------------------------------------
	// Login
	// --------------------------------------------------------------------------------------------

	Login(*models.RequestCtx)
}

var _ Controller = &wikistats.WikiStatsController{}
