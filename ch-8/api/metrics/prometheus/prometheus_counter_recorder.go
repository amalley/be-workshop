package prometheus

import (
	"github.com/amalley/be-workshop/ch-8/api/metrics"
	p "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusCounter struct {
	id      metrics.RecorderID
	counter p.Counter
}

func NewPrometheusCounter(name, help string, registry *p.Registry) *PrometheusCounter {
	return &PrometheusCounter{
		id: metrics.RecorderID(name),
		counter: promauto.With(registry).NewCounter(
			p.CounterOpts{
				Name: name,
				Help: help,
			},
		),
	}
}

func (r *PrometheusCounter) ID() metrics.RecorderID {
	return r.id
}

func (r *PrometheusCounter) Inc(value float64) {
	if value < 0 {
		return // Prometheus counters cannot be decremented
	}
	r.counter.Add(value)
}
