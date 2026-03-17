package middleware

import (
	"log/slog"
	"net/http"

	"github.com/AMalley/be-workshop/ch-3/api/utils"
)

func RequestLogger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := utils.SetCtxLogger(r.Context(), logger.With(
				"method", r.Method,
				"path", r.URL.Path,
			))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
