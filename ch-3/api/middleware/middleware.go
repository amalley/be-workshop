package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

type MiddlewareRegistry struct {
	stack []Middleware
}

func NewMiddlewareRegistry() *MiddlewareRegistry {
	return &MiddlewareRegistry{
		stack: make([]Middleware, 0),
	}
}

// Use registers a middleware processor to the registry stack.
//
// IMPORTANT: The order middleware is registered in, is the order they will execute in
func (mr *MiddlewareRegistry) Use(m Middleware) {
	mr.stack = append(mr.stack, m)
}

func (mr *MiddlewareRegistry) Resolve(root http.Handler) http.Handler {
	for i := len(mr.stack) - 1; i >= 0; i-- {
		root = mr.stack[i](root)
	}
	return root
}
