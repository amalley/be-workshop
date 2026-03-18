package producer

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/database"
	"github.com/amalley/be-workshop/ch-5/api/stream"
	"github.com/amalley/be-workshop/ch-5/api/web"
)

type ProducerController struct {
	logger *slog.Logger

	dbAdapter     database.Adapter
	streamAdapter stream.StreamAdapter
}

func NewProducerController(logger *slog.Logger, dbAdapter database.Adapter, streamAdapter stream.StreamAdapter) *ProducerController {
	return &ProducerController{
		logger:        logger.With(slog.String("src", "ProducerController")),
		dbAdapter:     dbAdapter,
		streamAdapter: streamAdapter,
	}
}

func (c *ProducerController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", web.HandleFunc(c.logger, c.Liveness))
}

// --------------------------------------------------------------------------------------------
// Lifecycle
// --------------------------------------------------------------------------------------------

func (c *ProducerController) OnStartup(ctx context.Context) error {
	c.logger.Info("ProducerController starting up...")

	// Connect to the database and stream in parallel to speed up startup time.
	// We will wait for the database to be ready before we start consuming the stream,
	// but we don't want to block the startup process while we wait for the stream to connect.
	var grp sync.WaitGroup
	grp.Go(func() {
		if err := c.dbAdapter.Connect(ctx); err != nil {
			c.logger.Error("failed to connect to database", "error", err.Error())
			return
		}
	})
	grp.Go(func() {
		if err := c.streamAdapter.Connect(ctx); err != nil {
			c.logger.Error("failed to connect to stream", "error", err.Error())
			return
		}
	})
	grp.Wait()

	wait := time.NewTicker(time.Second * 2)
	defer wait.Stop()

	// Ensure the database infrastructure is ready before we start consuming the stream
	for !c.dbAdapter.IsReady() {
		c.logger.Info("Waiting for database to be ready...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait.C:
		}
	}

	// Start consuming the stream in a separate goroutine so we don't block the startup process.
	// The server will be able to respond to health checks while we consume the stream.
	go func() {
		if err := c.streamAdapter.Consume(ctx); err != nil {
			c.logger.Error("stream consumption ended with error", "error", err.Error())
		}
	}()

	return nil
}

func (c *ProducerController) OnShutdown(ctx context.Context) error {
	c.logger.Info("ProducerController shutting down...")

	if err := c.streamAdapter.Close(ctx); err != nil {
		c.logger.Error("failed to close stream adapter", "error", err.Error())
	}

	if err := c.dbAdapter.Close(ctx); err != nil {
		c.logger.Error("failed to close database adapter", "error", err.Error())
	}

	return nil
}

// --------------------------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------------------------

func (c *ProducerController) Liveness(ctx *web.RequestCtx) {
	ctx.Send(http.StatusOK, []byte("OK"))
}
