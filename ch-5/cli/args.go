package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

type Args struct {
	Port       string
	URL        string
	LogLevel   string
	DBHost     string
	DBKeyspace string
}

func ParseArgs(defaults *Args) *Args {
	args := &Args{
		defaults.Port,
		defaults.URL,
		defaults.LogLevel,
		defaults.DBHost,
		defaults.DBKeyspace,
	}

	flag.StringVar(&args.Port, "port", args.Port, "Port to listen on")
	flag.StringVar(&args.URL, "url", args.URL, "WikiMedia stream url")
	flag.StringVar(&args.LogLevel, "log-level", args.LogLevel, "Application's log level (debug, info, warning, error)")
	flag.StringVar(&args.DBHost, "db-host", args.DBHost, "Database host")
	flag.StringVar(&args.DBKeyspace, "db-keyspace", args.DBKeyspace, "Database keyspace")
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
