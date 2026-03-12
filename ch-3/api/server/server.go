package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/AMalley/be-workshop/ch-3/api/utils"
)

const (
	ReadTimeout     = 10 * time.Second
	WriteTimeout    = 10 * time.Second
	IdleTimeout     = 120 * time.Second
	ShutdownTimeout = 10 * time.Second
)

// Controller defines the interface for the server's handler controller
type Controller interface {
	GetStats(writer http.ResponseWriter, req *http.Request)
	Liveness(writer http.ResponseWriter, req *http.Request)
}

// StreamAdapter defines the interface for an adapter to connect to and consume a stream
type StreamAdapter interface {
	Connect(ctx context.Context) error
	Consume(ctx context.Context) error
	Close(ctx context.Context) error
}

type DatabaseAdapter interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
}

// Middleware defines the signature of a request processor applied to each handler execution
type Middleware func(next http.Handler) http.Handler

// Server is the main structure of the application.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	stream   StreamAdapter
	database DatabaseAdapter

	logger *slog.Logger
	server *http.Server
	router *http.ServeMux

	tasks      sync.WaitGroup
	middleware []Middleware
	port       string
}

// NewServer returns a new instance of the WikiStats stream reader server.
func NewServer(logger *slog.Logger, stream StreamAdapter, database DatabaseAdapter, port string) *Server {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	router := http.NewServeMux()
	return &Server{
		ctx:      ctx,
		cancel:   cancel,
		stream:   stream,
		database: database,
		logger:   logger,
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      router,
			ReadTimeout:  ReadTimeout,
			WriteTimeout: WriteTimeout,
			IdleTimeout:  IdleTimeout,
		},
		router: router,
		port:   port,
	}
}

// RegisterHandlers registers the provided controller to handle the server endpoints
func (s *Server) RegisterHandlers(controller Controller) {
	s.router.Handle("GET /liveness", s.handler(controller.Liveness))
	s.router.Handle("GET /stats", s.handler(controller.GetStats))
}

// Use Middleware registers the provided middleware processor to the server.
// Middleware is resolved for each handler execution.
func (s *Server) UseMiddleware(middleware Middleware) {
	s.middleware = append(s.middleware, middleware)
}

// Start begins the core routines for the server - starting the http server, connecting the
// adapter to its stream, and starting the adapter's consumption loops.
func (s *Server) Start() {
	var grp sync.WaitGroup

	// Note: order of task completion is not guaranteed

	// Start the http server
	s.do(s.startServer)

	// Connect adapters to external services
	s.do(s.connectStream)
	s.do(s.connectDatabase)

	// Wait for shutdown
	<-s.ctx.Done()
	s.shutdown()

	grp.Wait()
}

// do does a task unless the context has been completed
func (s *Server) do(task func()) {
	if utils.CtxDone(s.ctx) {
		return
	}
	s.tasks.Go(task)
}

func (s *Server) startServer() {
	s.logger.Info("Starting server", slog.Any("port", s.port))

	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		s.logger.Error("Server failed to start", slog.Any("err", err))
		s.cancel()
	}
}

func (s *Server) connectDatabase() {
	s.logger.Info("Connecting database adapter...")

	err := s.database.Connect(s.ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("Database adapter failed to connect", slog.Any("err", err))
		s.cancel()
	}
}

func (s *Server) connectStream() {
	s.logger.Info("Connecting stream adapter...")

	err := s.stream.Connect(s.ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("Stream adapter failed to start", slog.Any("err", err))
		s.cancel()
		return
	}

	s.logger.Info("Consuming stream...")

	start := time.Now()

	err = s.stream.Consume(s.ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("Stream adapter failed to consume", slog.Any("err", err), slog.Any("duration", time.Since(start)))
		s.cancel()
	}
}

func (s *Server) shutdown() {
	s.logger.Info("Received shutdown signal")

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	if err := s.stream.Close(ctx); err != nil {
		s.logger.Error("Stream adapter failed to close", slog.Any("err", err))
	}

	if err := s.database.Close(ctx); err != nil {
		s.logger.Error("Database adapther failed to close", slog.Any("err", err))
	}

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server force to shutdown", slog.Any("err", err))
	}
}

func (s *Server) handler(handlerFunc http.HandlerFunc) http.Handler {
	var root http.Handler = handlerFunc

	// build the stack from FILO
	for i := len(s.middleware) - 1; i >= 0; i-- {
		root = s.middleware[i](root)
	}

	return root
}
