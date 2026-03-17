package models

import (
	"log/slog"
	"net/http"
)

type RequestCtx struct {
	logger   *slog.Logger
	request  *http.Request
	response http.ResponseWriter
}

func NewRequestCtx(logger *slog.Logger, w http.ResponseWriter, r *http.Request) *RequestCtx {
	return &RequestCtx{
		logger: logger.With(
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		),
		request:  r,
		response: w,
	}
}

func (c *RequestCtx) Logger() *slog.Logger {
	return c.logger
}

func (c *RequestCtx) Request() *http.Request {
	return c.request
}

func (c *RequestCtx) Response() http.ResponseWriter {
	return c.response
}
