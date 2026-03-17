package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// PanicRecover is a middleware that recovers from panics in the request handling chain.
// If a panic occurs, it logs the error and stack trace, and returns a 500 Internal Server Error response.
func PanicRecover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("PANIC RECOVER",
						slog.Any("err", err),
						slog.String("stack", string(debug.Stack())),
					)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("internal server error"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
