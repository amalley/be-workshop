package controller

import (
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/controller/wikistats"
)

// Controller defines the interface for the server's handler controller
type Controller interface {
	CreateUser(w http.ResponseWriter, r *http.Request)
	DeleteUser(w http.ResponseWriter, r *http.Request)

	GetStats(w http.ResponseWriter, r *http.Request)

	Liveness(w http.ResponseWriter, r *http.Request)
	Readiness(w http.ResponseWriter, r *http.Request)
}

var _ Controller = &wikistats.WikiStatsController{}
