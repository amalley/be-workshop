package controller

import (
	"net/http"
)

// Controller defines the interface for the server's handler controller
type Controller interface {
	GetStats(writer http.ResponseWriter, req *http.Request)
	Liveness(writer http.ResponseWriter, req *http.Request)
	Readiness(writer http.ResponseWriter, req *http.Request)
}
