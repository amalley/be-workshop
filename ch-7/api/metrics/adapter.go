package metrics

import (
	"errors"

	"github.com/amalley/be-workshop/ch-7/api/web"
)

var (
	ErrRecorderNotFound    = errors.New("recorder not found")
	ErrInvalidRecorderType = errors.New("invalid recorder type")
)

type RecorderTable map[RecorderID]Recorder

type Adapter interface {
	HttpHandler(ctx *web.RequestCtx)
	AddRecorders(recorders ...Recorder)
	Increment(recorderID RecorderID, value float64) error
}
