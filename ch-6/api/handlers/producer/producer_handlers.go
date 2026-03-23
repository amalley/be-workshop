package producer

import (
	"log/slog"
	"net/http"

	"github.com/amalley/be-workshop/ch-6/api/handlers"
	"github.com/amalley/be-workshop/ch-6/api/web"
)

// Ensure ProducerHandlers implements all required handler interfaces.
var _ handlers.HealthHandlers = &ProducerHandlers{}

type ProducerHandlers struct {
	logger *slog.Logger
}

func NewProducerHandlers(logger *slog.Logger) *ProducerHandlers {
	return &ProducerHandlers{
		logger: logger,
	}
}

func (h *ProducerHandlers) RegisterHandlers(mux *http.ServeMux) {
	mux.Handle("GET /liveness", web.WithRequestCtx(h.logger, h.Liveness))
}

// --------------------------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------------------------

func (c *ProducerHandlers) Liveness(ctx *web.RequestCtx) {
	ctx.Send(http.StatusOK, []byte("OK"))
}
