package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/authentication/public"
	"github.com/amalley/be-workshop/ch-5/api/database"
	"github.com/amalley/be-workshop/ch-5/api/database/scylla"
	"github.com/amalley/be-workshop/ch-5/api/handlers/consumer"
	"github.com/amalley/be-workshop/ch-5/api/middleware"
	"github.com/amalley/be-workshop/ch-5/api/server"
	"github.com/amalley/be-workshop/ch-5/api/utils"
	"github.com/amalley/be-workshop/ch-5/cli"
	"github.com/gocql/gocql"
)

func main() {
	args := cli.ParseArgs(&cli.Args{
		Port:       cli.GetEnv("PORT", "7000"),
		LogLevel:   cli.GetEnv("LOG_LEVEL", "debug"),
		DBHost:     cli.GetEnv("DB_HOST", "scylla"),
		DBKeyspace: cli.GetEnv("DB_KEYSPACE", "wikistats"),
	})

	mux := http.NewServeMux()
	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cli.ParseLogLevel(args.LogLevel),
	})).With("svc", "wikistats-consumer")

	syl := scylla.NewScyllaDatabaseAdapter(
		scylla.WithLogger(lgr),
		scylla.WithHost(args.DBHost),
		scylla.WithKeyspace(args.DBKeyspace),
		scylla.WithClusterConsistency(gocql.Quorum),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
	)

	pub := public.NewPublicAuthenticator(lgr)

	hdl := consumer.NewConsumerHandlers(lgr, syl, pub)
	hdl.RegisterHandlers(mux)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(middleware.PanicRecover(lgr))

	server.NewServer(
		server.WithAddress(":"+args.Port),
		server.WithHandler(mdl.Resolve(mux)),
		server.WithLogger(lgr),
		server.WithStartupHook(startup(lgr, syl)),
		server.WithShutdownHook(shutdown(lgr, syl)),
	).Run(ctx)
}

func startup(logger *slog.Logger, dbAdapter database.Adapter) server.ServerHook {
	return func(ctx context.Context) error {
		if utils.CtxDone(ctx) {
			logger.Info("failed to start stream", slog.String("reason", ctx.Err().Error()))
			return ctx.Err()
		}

		var grp sync.WaitGroup
		grp.Go(func() {
			if err := dbAdapter.Connect(ctx); err != nil {
				logger.Error("Failed to connect to database", slog.Any("err", err))
			}
		})
		grp.Wait()

		wait := time.NewTicker(time.Second * 2)
		defer wait.Stop()

		for !dbAdapter.IsReady() {
			logger.Info("Waiting for database to be ready...")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-wait.C:
			}
		}

		_, exists, err := dbAdapter.GetUser(ctx, "admin")
		if err != nil {
			logger.Error("Failed to get default user", slog.Any("err", err))
		}

		if !exists {
			// Don't do this in production
			if err := dbAdapter.CreateUser(ctx, "admin", "password"); err != nil {
				logger.Error("Failed to create default user", slog.Any("err", err))
			}
		}

		return nil
	}
}

func shutdown(logger *slog.Logger, dbAdapter database.Adapter) server.ServerHook {
	return func(ctx context.Context) error {
		if utils.CtxDone(ctx) {
			logger.Info("failed to shutdown", slog.String("reason", ctx.Err().Error()))
			return ctx.Err()
		}

		if err := dbAdapter.Close(ctx); err != nil {
			logger.Error("Failed to close database", slog.Any("err", err))
		}

		return nil
	}
}
