package handlers

import "github.com/amalley/be-workshop/ch-5/api/web"

// HealthHandlers defines the interface for handling health check requests.
type HealthHandlers interface {
	Liveness(*web.RequestCtx)
}
