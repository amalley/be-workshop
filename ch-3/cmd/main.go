package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/AMalley/be-workshop/ch-3/api/adapter"
	"github.com/AMalley/be-workshop/ch-3/api/controller"
	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/api/server"
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

func mustGetEnv(name string) string {
	v, exists := os.LookupEnv(name)
	if !exists {
		panic(fmt.Sprintf("missing required env variable: %s", name))
	}
	return v
}

func main() {
	args := parseArgs()

	// Create application components
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(args.LogLevel),
	}))
	db := database.NewWikiStatsDB()
	syl := adapter.NewScyllaDatabaseAdapter(logger, mustGetEnv("SCYLLA_HOST"))
	stm := adapter.NewWikiStreamAdapter(logger, db, args.URL)
	ctl := controller.NewController(logger, db)

	// Create the server and register handlers
	svr := server.NewServer(logger, stm, syl, args.Port)
	svr.RegisterHandlers(ctl)

	// Start the server
	svr.Start()
}
