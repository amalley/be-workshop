package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/amalley/be-workshop/ch-5/cli"
)

func main() {
	args := cli.ParseArgs()

	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cli.ParseLogLevel(args.LogLevel),
	})).With("app", "wikistats-producer")

	lgr.Info("Hello, world!")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	<-ctx.Done()
	lgr.Info("Shutting down...")
}
