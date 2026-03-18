package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/authentication/public"
	"github.com/amalley/be-workshop/ch-5/api/controller/consumer"
	"github.com/amalley/be-workshop/ch-5/api/database/scylla"
	"github.com/amalley/be-workshop/ch-5/api/middleware"
	"github.com/amalley/be-workshop/ch-5/api/server"
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

	syl := scylla.NewScyllaDatabaseAdapter(lgr,
		scylla.WithHost(args.DBHost),
		scylla.WithKeyspace(args.DBKeyspace),
		scylla.WithClusterConsistency(gocql.Quorum),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
	)

	pub := public.NewPublicAuthenticator(lgr)

	ctl := consumer.NewConsumerController(lgr, syl, pub)
	ctl.RegisterRoutes(mux)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(middleware.PanicRecover(lgr))

	server.NewServer(
		server.WithAddress(":"+args.Port),
		server.WithHandler(mdl.Resolve(mux)),
		server.WithLogger(lgr),
		server.WithStartupHook(ctl.OnStartup),
		server.WithShutdownHook(ctl.OnShutdown),
	).Run(ctx)
}
