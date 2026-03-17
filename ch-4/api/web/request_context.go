package web

import (
	"log/slog"
	"net/http"
)

// RequestCtx encapsulates the context of an HTTP request, providing access to the logger, request, and response writer.
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

func (c *RequestCtx) SendError(statusCode int, err error) {
	SendError(c.logger, c.response, statusCode, err)
}

func (c *RequestCtx) Send(statusCode int, body []byte) {
	Send(c.logger, c.response, statusCode, body)
}

func (c *RequestCtx) SendWithHeaders(statusCode int, body []byte, headers map[string]string) {
	SendWithHeaders(c.logger, c.response, statusCode, body, headers)
}

func (c *RequestCtx) SendJSON(statusCode int, data any) {
	SendJSON(c.logger, c.response, statusCode, data)
}

func (c *RequestCtx) SendJSONWithHeaders(statusCode int, data any, headers map[string]string) {
	SendJSONWithHeaders(c.logger, c.response, statusCode, data, headers)
}
