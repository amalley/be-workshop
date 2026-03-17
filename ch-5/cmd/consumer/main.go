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
	"github.com/amalley/be-workshop/ch-5/api/controller/wikistats"
	"github.com/amalley/be-workshop/ch-5/api/database/scylla"
	"github.com/amalley/be-workshop/ch-5/api/middleware"
	"github.com/amalley/be-workshop/ch-5/api/server"
	"github.com/amalley/be-workshop/ch-5/cli"
	"github.com/gocql/gocql"
)

func main() {
	args := cli.ParseArgs()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mux := http.NewServeMux()
	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cli.ParseLogLevel(args.LogLevel),
	}))

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(middleware.PanicRecover(lgr))

	syl := scylla.NewScyllaDatabaseAdapter(lgr,
		scylla.WithHost("scylla"),
		scylla.WithKeyspace("wikistats"),
		scylla.WithClusterConsistency(gocql.Quorum),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
	)

	pub := public.NewPublicAuthenticator(lgr)

	ctl := wikistats.NewWikiStatsController(lgr, syl, pub)
	ctl.RegisterRoutes(mux)

	server.NewServer(
		server.WithAddress(":"+args.Port),
		server.WithHandler(mdl.Resolve(mux)),
		server.WithLogger(lgr),
		server.WithStartupHook(ctl.OnStartup),
		server.WithShutdownHook(ctl.OnShutdown),
	).Run(ctx)
}

// u, err := url.Parse(args.URL)
// if err != nil {
// 	lgr.Error("fatal: unable to parse stream URL", "url", args.URL, "error", err.Error())
// 	os.Exit(1)
// }

// stm := wiki.NewWikiStreamAdapter(lgr, syl, u)

// grp.Go(func() {
// 	if err := c.stream.Connect(ctx); err != nil {
// 		c.logger.Error("Failed to connect to stream", slog.Any("err", err))
// 	}
// })
// go func() {
// 	c.logger.Info("Starting stream consumption")
// 	if err := c.stream.Consume(ctx); err != nil && !errors.Is(err, context.Canceled) {
// 		c.logger.Error("Failed to consume stream", slog.Any("err", err))
// 	}
// }()
