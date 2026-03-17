package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// SendError logs the error and sends an HTTP error response with the specified status code and error message.
func SendError(logger *slog.Logger, w http.ResponseWriter, statusCode int, err error) {
	logger.Error("Request error", slog.Any("err", err))
	http.Error(w, err.Error(), statusCode)
}

// SendJSON encodes the provided data as JSON and sends it in the HTTP response with the specified status code.
func SendJSON(logger *slog.Logger, w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("failed to write JSON response", slog.Any("err", err))
	}
}

// SendJSONWithHeaders encodes the provided data as JSON, sets additional headers, and sends it in the HTTP response with the specified status code.
func SendJSONWithHeaders(logger *slog.Logger, w http.ResponseWriter, statusCode int, data any, headers map[string]string) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	SendJSON(logger, w, statusCode, data)
}

// Send logs the response and sends an HTTP response with the specified status code and body.
func Send(logger *slog.Logger, w http.ResponseWriter, statusCode int, body []byte) {
	w.WriteHeader(statusCode)

	_, err := w.Write(body)
	if err != nil {
		logger.Error("failed to write response", slog.Any("err", err))
	}
}

// SendWithHeaders sets additional headers and sends an HTTP response with the specified status code and body.
func SendWithHeaders(logger *slog.Logger, w http.ResponseWriter, statusCode int, body []byte, headers map[string]string) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	Send(logger, w, statusCode, body)
}
