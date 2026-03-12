package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/AMalley/be-workshop/ch-2/api/adapter"
	"github.com/AMalley/be-workshop/ch-2/api/controller"
	"github.com/AMalley/be-workshop/ch-2/api/database"
	"github.com/AMalley/be-workshop/ch-2/api/server"
)

type Args struct {
	Port     int
	URL      string
	LogLevel string
}

func parseArgs() *Args {
	args := &Args{
		Port:     7000,
		URL:      "https://stream.wikimedia.org/v2/stream/recentchange",
		LogLevel: "info",
	}

	flag.IntVar(&args.Port, "port", args.Port, "Port to listen on")
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

func main() {
	args := parseArgs()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(args.LogLevel),
	}))
	db := database.NewWikiStatsDB()

	// Initialize the server and its components
	adp := adapter.NewWikiStreamAdapter(logger, db, args.URL)
	ctl := controller.NewController(logger, db)
	svr := server.NewServer(logger, ctl, adp, args.Port)

	// Start the server
	svr.Start()
}
