package metrics

import (
	"errors"
	"net/http"
)

var (
	ErrRecorderNotFound    = errors.New("recorder not found")
	ErrInvalidRecorderType = errors.New("invalid recorder type")
)

type RecorderTable map[RecorderID]Recorder

type Adapter interface {
	HttpHandler(w http.ResponseWriter, r *http.Request)
	AddRecorders(recorders ...Recorder)
	Increment(recorderID RecorderID, value float64) error
}
