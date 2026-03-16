package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AMalley/be-workshop/ch-3/api/authentication/public"
	"github.com/AMalley/be-workshop/ch-3/api/controller/wikistats"
	"github.com/AMalley/be-workshop/ch-3/api/database/scylla"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
	"github.com/AMalley/be-workshop/ch-3/api/server"
	"github.com/AMalley/be-workshop/ch-3/api/stream/wiki"
	"github.com/gocql/gocql"
)

type Args struct {
	Port     string
	URL      string
	LogLevel string
}

func parseArgs() *Args {
	args := &Args{
		Port:     getEnv("PORT", "7000"),
		URL:      getEnv("STREAM_URL", "https://stream.wikimedia.org/v2/stream/recentchange"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	flag.StringVar(&args.Port, "port", args.Port, "Port to listen on")
	flag.StringVar(&args.URL, "url", args.URL, "WikiMedia stream url")
	flag.StringVar(&args.LogLevel, "log-level", args.LogLevel, "Application's log level (debug, info, warning, error)")
	flag.Parse()

	return args
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		panic(fmt.Errorf("unknown log level: %s", level))
	}
}

func getEnv(name string, fallback string) string {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	return v
}

func main() {
	args := parseArgs()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mux := http.NewServeMux()
	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(args.LogLevel),
	}))

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(middleware.RequestLogger(lgr))
	mdl.Use(middleware.PanicRecover(lgr))

	syl := scylla.NewScyllaDatabaseAdapter(lgr,
		scylla.WithHost("scylla"),
		scylla.WithKeyspace("wikistats"),
		scylla.WithClusterConsistency(gocql.Quorum),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
	)

	stm := wiki.NewWikiStreamAdapter(lgr, syl, args.URL)
	pub := public.NewPublicAuthenticator()

	ctl := wikistats.NewWikiStatsController(lgr, stm, syl, pub)
	ctl.RegisterRoutes(mux)

	server.NewServer(
		server.WithAddress(":"+args.Port),
		server.WithHandler(mdl.Resolve(mux)),
		server.WithLogger(lgr),
		server.WithStartupHook(ctl.OnStartup),
		server.WithShutdownHook(ctl.OnShutdown),
	).Run(ctx)
}
