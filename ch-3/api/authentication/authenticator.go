package authentication

import (
	"context"

	"github.com/AMalley/be-workshop/ch-3/api/middleware"
)

type ctxUserIDKey struct{}

// Authenticator defines the interface for handling authentication.
type Authenticator interface {
	AuthenticationMiddleware(subVerify func(sub string) bool) middleware.Middleware
	GenerateToken(iss, sub string) (string, error)
}

// SetCtxUserID sets the user ID in the context, allowing it to be accessed in handlers after authentication.
func SetCtxUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxUserIDKey{}, userID)
}

// GetCtxUserID retrieves the user ID from the context, returning it along with a boolean indicating if it was found.
func GetCtxUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ctxUserIDKey{}).(string)
	return userID, ok
}
