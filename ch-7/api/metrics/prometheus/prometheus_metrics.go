package prometheus

import (
	"log/slog"
	"net/http"

	"github.com/amalley/be-workshop/ch-7/api/metrics"
	"github.com/amalley/be-workshop/ch-7/api/web"
	p "github.com/prometheus/client_golang/prometheus"
)

var _ metrics.Adapter = &PrometheusAdapter{}

type PrometheusAdapter struct {
	logger   *slog.Logger
	registry *p.Registry

	recorders metrics.RecorderTable
}

func NewPrometheusAdapter(logger *slog.Logger) *PrometheusAdapter {
	return &PrometheusAdapter{
		logger:    logger.With("src", "PrometheusAdapter"),
		registry:  p.NewRegistry(),
		recorders: make(metrics.RecorderTable),
	}
}

func (a *PrometheusAdapter) Registry() *p.Registry {
	return a.registry
}

func (a *PrometheusAdapter) HttpHandler(ctx *web.RequestCtx) {
	ctx.Send(http.StatusOK, []byte(`TODO: Implement Prometheus metrics endpoint`))
}

func (a *PrometheusAdapter) AddRecorders(recorders ...metrics.Recorder) {
	for _, recorder := range recorders {
		if _, exists := a.recorders[recorder.ID()]; exists {
			a.logger.Warn("Recorder with ID already exists, skipping", "recorder_id", recorder.ID())
			continue
		}

		a.recorders[recorder.ID()] = recorder
		a.logger.Info("Added recorder", "recorder_id", recorder.ID())
	}
}

func (a *PrometheusAdapter) Increment(recorderID metrics.RecorderID, value float64) error {
	recorder, exists := a.recorders[recorderID]
	if !exists {
		a.logger.Error("Recorder not found", "recorder_id", recorderID)
		return metrics.ErrRecorderNotFound
	}

	counter, ok := recorder.(*PrometheusCounter)
	if !ok {
		a.logger.Error("Recorder is not a PrometheusCounter", "recorder_id", recorderID)
		return metrics.ErrInvalidRecorderType
	}

	counter.Inc(value)
	return nil
}
