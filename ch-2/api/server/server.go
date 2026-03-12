package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	ReadTimeout     = 10 * time.Second
	WriteTimeout    = 10 * time.Second
	IdleTimeout     = 120 * time.Second
	ShutdownTimeout = 10 * time.Second
)

type Controller interface {
	GetStats(writer http.ResponseWriter, req *http.Request)
	Liveness(writer http.ResponseWriter, req *http.Request)
}

type StreamAdapter interface {
	Connect(ctx context.Context) error
	Consume(ctx context.Context) error
	Close(ctx context.Context) error
}

type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	controller Controller
	adapter    StreamAdapter

	logger *slog.Logger
	server *http.Server

	port int
}

func NewServer(logger *slog.Logger, controller Controller, adapter StreamAdapter, port int) *Server {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	router := http.NewServeMux()
	registerHandlers(router, controller)

	return &Server{
		ctx:        ctx,
		cancel:     cancel,
		controller: controller,
		adapter:    adapter,
		logger:     logger,
		server: &http.Server{
			Addr:         ":" + strconv.Itoa(port),
			Handler:      router,
			ReadTimeout:  ReadTimeout,
			WriteTimeout: WriteTimeout,
			IdleTimeout:  IdleTimeout,
		},
		port: port,
	}
}

// Start begins the core routines for the server - starting the http server, connecting the
// adapter to its stream, and starting the adapter's consumption loops.
func (s *Server) Start() {
	var grp sync.WaitGroup

	s.startServer(&grp)
	s.startAdapter(&grp)

	<-s.ctx.Done()
	s.shutdown()

	grp.Wait()
}

func (s *Server) startServer(grp *sync.WaitGroup) {
	s.logger.Info("Starting server", slog.Any("port", s.port))
	grp.Go(func() {
		err := s.server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
			s.logger.Error("Server failed to start", slog.Any("err", err))
			s.cancel()
		}
	})
}

func (s *Server) startAdapter(grp *sync.WaitGroup) {
	s.logger.Info("Starting stream adapter")
	grp.Go(func() {
		start := time.Now()

		err := s.adapter.Connect(s.ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Error("Stream adapter failed to start", slog.Any("err", err))
			s.cancel()
			return
		}

		err = s.adapter.Consume(s.ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Error("Stream adapter failed to consume", slog.Any("err", err), slog.Any("duration", time.Since(start)))
			s.cancel()
		}
	})
}

func (s *Server) shutdown() {
	s.logger.Info("Received shutdown signal")

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	if err := s.adapter.Close(ctx); err != nil {
		s.logger.Error("Stream adapter failed to close", slog.Any("err", err))
	}

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server force to shutdown", slog.Any("err", err))
	}
}

func registerHandlers(router *http.ServeMux, controller Controller) {
	router.HandleFunc("/liveness", controller.Liveness)
	router.HandleFunc("/stats", controller.GetStats)
}
