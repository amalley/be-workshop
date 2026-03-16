package authentication

import (
	"github.com/AMalley/be-workshop/ch-3/api/authentication/public"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
)

type Authenticator interface {
	AuthenticationMiddleware(subVerify func(sub string) bool) middleware.Middleware
	GenerateToken(sub string) (string, error)
}

var _ Authenticator = &public.PublicAuthenticator{}
