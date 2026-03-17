package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

type Args struct {
	Port     string
	URL      string
	LogLevel string
}

func ParseArgs() *Args {
	args := &Args{
		Port:     GetEnv("PORT", "7000"),
		URL:      GetEnv("STREAM_URL", "https://stream.wikimedia.org/v2/stream/recentchange"),
		LogLevel: GetEnv("LOG_LEVEL", "info"),
	}

	flag.StringVar(&args.Port, "port", args.Port, "Port to listen on")
	flag.StringVar(&args.URL, "url", args.URL, "WikiMedia stream url")
	flag.StringVar(&args.LogLevel, "log-level", args.LogLevel, "Application's log level (debug, info, warning, error)")
	flag.Parse()

	return args
}

func ParseLogLevel(level string) slog.Level {
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

func GetEnv(name string, fallback string) string {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	return v
}
