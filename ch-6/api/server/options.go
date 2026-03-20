package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Option is a functional setter for a server configuration option.
type Option func(*Options)

// Options are the configuration options for a server.
type Options struct {
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	Handler           http.Handler
	Address           string
	Logger            *slog.Logger
	StartupHook       ServerHook
	ShutdownHook      ServerHook
}

func WithReadTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.ReadTimeout = timeout
	}
}

func WithReadHeaderTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.ReadHeaderTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.WriteTimeout = timeout
	}
}

func WithIdleTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.IdleTimeout = timeout
	}
}

func WithShutdownTimout(timeout time.Duration) Option {
	return func(o *Options) {
		o.ShutdownTimeout = timeout
	}
}

func WithHandler(handler http.Handler) Option {
	return func(o *Options) {
		o.Handler = handler
	}
}

func WithAddress(address string) Option {
	return func(o *Options) {
		o.Address = address
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

func WithStartupHook(hook ServerHook) Option {
	return func(o *Options) {
		o.StartupHook = hook
	}
}

func WithShutdownHook(hook ServerHook) Option {
	return func(o *Options) {
		o.ShutdownHook = hook
	}
}

func defaultOptions() *Options {
	return &Options{
		ShutdownTimeout:   5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		Logger:            slog.Default(),
		Handler:           http.NotFoundHandler(),
		Address:           ":7000",
		StartupHook:       func(ctx context.Context) error { return nil },
		ShutdownHook:      func(ctx context.Context) error { return nil },
	}
}

func applyOptions(cfg *Options, opts ...Option) *Options {
	for i := range opts {
		opts[i](cfg)
	}
	return cfg
}
