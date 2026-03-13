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

	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
	"github.com/AMalley/be-workshop/ch-3/api/stream"
	"github.com/AMalley/be-workshop/ch-3/api/utils"
)

const (
	ReadTimeout     = 10 * time.Second
	WriteTimeout    = 10 * time.Second
	IdleTimeout     = 120 * time.Second
	ShutdownTimeout = 10 * time.Second
)

// Server is the main structure of the application.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	stream   stream.StreamAdapter
	database database.DatabaseAdpater

	logger *slog.Logger
	server *http.Server

	tasks sync.WaitGroup
	port  string
}

// NewServer returns a new instance of the WikiStats stream reader server.
func NewServer(logger *slog.Logger, router *http.ServeMux, stream stream.StreamAdapter, database database.DatabaseAdpater, port string) *Server {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return &Server{
		ctx:      ctx,
		cancel:   cancel,
		stream:   stream,
		database: database,
		logger:   logger.With(slog.String("src", "Server")),
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      router,
			ReadTimeout:  ReadTimeout,
			WriteTimeout: WriteTimeout,
			IdleTimeout:  IdleTimeout,
		},
		port: port,
	}
}

// ContextCancelledMiddleware ensures handlers are not called if the server context has been cancelled.
func (s *Server) ContextCancelledMiddleware() middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if utils.CtxDone(s.ctx) {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Handler is a wrapper http.Handler that ensure requests are handled only if the server context is still alive.
func (s *Server) Handler(handlerFunc http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if utils.CtxDone(s.ctx) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		handlerFunc.ServeHTTP(w, r)
	})
}

// Start begins the core routines for the server - starting the http server, connecting the
// adapter to its stream, and starting the adapter's consumption loops.
func (s *Server) Start() {
	s.do(s.connectDatabase)
	s.tasks.Wait() // We'll wait for the database connection to be establisted first.

	s.do(s.startServer)
	s.do(s.connectStream)

	<-s.ctx.Done()
	s.shutdown()

	s.tasks.Wait()
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

	s.do(s.consumeStream)
}

func (s *Server) consumeStream() {
	s.logger.Info("Consuming stream...")

	start := time.Now()

	err := s.stream.Consume(s.ctx)
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

	// Scylla is overflowing the stack, fix this later
	// if err := s.database.Close(ctx); err != nil {
	// 	s.logger.Error("Database adapther failed to close", slog.Any("err", err))
	// }

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server force to shutdown", slog.Any("err", err))
	}
}
