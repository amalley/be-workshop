package middleware

import (
	"context"
	"log/slog"
	"net/http"
)

func RequestLogger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "logger", logger.With(
				"method", r.Method,
				"path", r.URL.Path,
			))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
