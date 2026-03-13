package public

import (
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/middleware"
)

type PublicAuthenticator struct {
}

func NewPublicAuthenticator() *PublicAuthenticator {
	return &PublicAuthenticator{}
}

func (p *PublicAuthenticator) AuthorizationMiddleware() middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func (p *PublicAuthenticator) GenerateToken(sub string) (string, error) {
	return "", nil
}
