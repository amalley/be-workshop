package utils

import (
	"context"
	"log/slog"
)

type ctxLoggerKey struct{}

// SetCtxLogger sets a logger in the context, which can be retrieved using GetCtxLogger.
func SetCtxLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, logger)
}

// GetCtxLogger retrieves a logger from the context, if one exists.
func GetCtxLogger(ctx context.Context) (*slog.Logger, bool) {
	l, ok := ctx.Value(ctxLoggerKey{}).(*slog.Logger)
	return l, ok
}

// CtxErr checks if the context is done before returning the provided err.
// If the context is done, then its err is returned instead.
func CtxErr(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return err
	}
}

// CtxDone checks if the context is done without blocking.
func CtxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
