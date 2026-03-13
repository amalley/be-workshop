package utils

import "context"

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

func CtxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
