package handlers

import "github.com/amalley/be-workshop/ch-9/api/web"

// LoginHandlers defines the interface for handling login requests.
type LoginHandlers interface {
	Login(*web.RequestCtx)
}
