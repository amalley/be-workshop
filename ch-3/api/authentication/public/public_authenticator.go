package public

import (
	"net/http"
	"time"

	"github.com/AMalley/be-workshop/ch-3/api/authentication"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
	"github.com/golang-jwt/jwt/v5"
)

var _ authentication.Authenticator = &PublicAuthenticator{}

type PublicAuthenticator struct {
}

func NewPublicAuthenticator() *PublicAuthenticator {
	return &PublicAuthenticator{}
}

func (p *PublicAuthenticator) AuthenticationMiddleware(subVerify func(sub string) bool) middleware.Middleware {
	if subVerify == nil {
		subVerify = func(sub string) bool { return false }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("missing authorization header"))
				return
			}

			tokenStr := authHeader[len("Bearer "):]

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte("secret"), nil
			})

			if err != nil || !token.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("invalid token"))
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("invalid token claims"))
				return
			}

			exp, err := claims.GetExpirationTime()
			if err != nil || time.Unix(exp.Unix(), 0).Before(time.Now()) {
				w.WriteHeader(http.StatusUnauthorized)
				if err != nil {
					w.Write([]byte("invalid token expiration"))
				} else {
					w.Write([]byte("token expired"))
				}
				return
			}

			sub, err := claims.GetSubject()
			if err != nil || !subVerify(sub) {
				w.WriteHeader(http.StatusUnauthorized)
				if err != nil {
					w.Write([]byte("invalid token subject"))
				} else {
					w.Write([]byte("unauthorized subject"))
				}
				return
			}

			ctx := authentication.SetCtxUserID(r.Context(), sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GenerateToken generates a jwt token for the given subject.
func (p *PublicAuthenticator) GenerateToken(sub string) (string, error) {
	claim := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub,
		"iss": "wikistats-app",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	return claim.SignedString([]byte("secret"))
}
