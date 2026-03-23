package public

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/amalley/be-workshop/ch-8/api/authentication"
	"github.com/amalley/be-workshop/ch-8/api/middleware"
	"github.com/amalley/be-workshop/ch-8/api/web"
	"github.com/golang-jwt/jwt/v5"
)

var _ authentication.Authenticator = &PublicAuthenticator{}

type PublicAuthenticator struct {
	logger *slog.Logger
}

func NewPublicAuthenticator(logger *slog.Logger) *PublicAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	return &PublicAuthenticator{
		logger: logger,
	}
}

func (p *PublicAuthenticator) AuthenticationMiddleware(subVerifier authentication.SubjectVerifier) middleware.Middleware {
	if subVerifier == nil {
		// Default subject verifier that rejects all subjects.
		// We'll assume no one is authorized if a verifier wasn't provided.
		subVerifier = func(sub string) bool { return false }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("missing Authorization header"))
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
				web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("invalid token"))
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("invalid token claims"))
				return
			}

			exp, err := claims.GetExpirationTime()
			if err != nil || time.Unix(exp.Unix(), 0).Before(time.Now()) {
				if err != nil {
					web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("invalid token expiration"))
				} else {
					web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("token expired"))
				}
				return
			}

			sub, err := claims.GetSubject()
			if err != nil || !subVerifier(sub) {
				if err != nil {
					web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("invalid token subject"))
				} else {
					web.SendError(p.logger, w, http.StatusUnauthorized, errors.New("unauthorized subject"))
				}
				return
			}

			ctx := authentication.SetCtxUserID(r.Context(), sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GenerateToken generates a jwt token for the given subject.
func (p *PublicAuthenticator) GenerateToken(iss, sub string) (string, error) {
	claim := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub,
		"iss": iss,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	return claim.SignedString([]byte("secret"))
}
