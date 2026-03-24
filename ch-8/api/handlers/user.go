package handlers

import "github.com/amalley/be-workshop/ch-8/api/web"

// UserHandlers defines the interface for handling user-related requests.
type UserHandlers interface {
	CreateUser(*web.RequestCtx)
	DeleteUser(*web.RequestCtx)
}
