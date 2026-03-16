package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func PanicRecover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l, ok := r.Context().Value("logger").(*slog.Logger)
			if !ok {
				l = logger
			}
			defer func() {
				if err := recover(); err != nil {
					l.Error("PANIC RECOVER",
						slog.Any("err", err),
						slog.String("stack", string(debug.Stack())),
					)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
