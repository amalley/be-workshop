package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/amalley/be-workshop/ch-8/api/utils"
)

type ServerHook func(ctx context.Context) error

type Server struct {
	cfg *Options
	svr *http.Server
}

func NewServer(opts ...Option) *Server {
	cfg := applyOptions(defaultOptions(), opts...)
	return &Server{
		cfg: cfg,
		svr: &http.Server{
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			Handler:           cfg.Handler,
			Addr:              cfg.Address,
		},
	}
}

func (s *Server) Run(ctx context.Context) {
	if utils.CtxDone(ctx) {
		s.cfg.Logger.Error("run failed", "reason", ctx.Err().Error())
		return
	}

	s.cfg.Logger.Info("starting server", "address", s.cfg.Address)
	go func() {
		if err := s.svr.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.cfg.Logger.Error("server error", "err", err.Error())
		}
	}()

	if err := s.cfg.StartupHook(ctx); err != nil {
		s.cfg.Logger.Error("startup hook error", "err", err.Error())
	}

	<-ctx.Done()
	s.cfg.Logger.Info("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer shutdownCancel()

	s.shutdown(shutdownCtx)
}

func (s *Server) shutdown(ctx context.Context) {
	if utils.CtxDone(ctx) {
		s.cfg.Logger.Error("shutdown failed", "reason", ctx.Err().Error())
		return
	}

	if err := s.cfg.ShutdownHook(ctx); err != nil {
		s.cfg.Logger.Error("shutdown hook error", "err", err.Error())
	}

	if err := s.svr.Shutdown(ctx); err != nil {
		s.cfg.Logger.Error("shutdown error", "err", err.Error())
	}
}
