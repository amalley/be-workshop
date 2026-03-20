package handlers

import "github.com/amalley/be-workshop/ch-5/api/web"

// StatsHandlers defines the interface for handling statistics requests.
type StatsHandlers interface {
	GetStats(*web.RequestCtx)
}
