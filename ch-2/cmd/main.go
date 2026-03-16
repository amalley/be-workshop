package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AMalley/be-workshop/ch-2/api/adapter"
	"github.com/AMalley/be-workshop/ch-2/api/controller"
	"github.com/AMalley/be-workshop/ch-2/api/database"
	"github.com/AMalley/be-workshop/ch-2/api/server"
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

func loadDotEnv() {
	envFile, err := os.Open(".env")
	if err != nil {
		println(fmt.Sprintf("error loading .env: %s", err.Error()))
		return // We'll log the error and continue, our arguments have fallbacks and this shouldn't kill the application.
	}
	defer envFile.Close()

	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		line := scanner.Text()
		kvp := strings.Split(line, "=")

		if len(kvp) != 2 {
			continue // Malformed .env line, skip it
		}

		os.Setenv(kvp[0], kvp[1])
	}
}

func main() {
	loadDotEnv()

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
