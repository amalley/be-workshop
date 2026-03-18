package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

type Args struct {
	Port               string
	URL                string
	LogLevel           string
	DBHost             string
	DBKeyspace         string
	KafkaBrokers       string
	KafkaTopic         string
	KafkaRetryAttempts int
}

func ParseArgs(defaults *Args) *Args {
	flag.StringVar(&defaults.Port, "port", defaults.Port, "Port to listen on")
	flag.StringVar(&defaults.URL, "url", defaults.URL, "WikiMedia stream url")
	flag.StringVar(&defaults.LogLevel, "log-level", defaults.LogLevel, "Application's log level (debug, info, warning, error)")
	flag.StringVar(&defaults.DBHost, "db-host", defaults.DBHost, "Database host")
	flag.StringVar(&defaults.DBKeyspace, "db-keyspace", defaults.DBKeyspace, "Database keyspace")
	flag.StringVar(&defaults.KafkaBrokers, "kafka-brokers", defaults.KafkaBrokers, "Comma-separated list of Kafka brokers")
	flag.StringVar(&defaults.KafkaTopic, "kafka-topic", defaults.KafkaTopic, "Kafka topic to produce to / consume from")
	flag.IntVar(&defaults.KafkaRetryAttempts, "kafka-retry-attempts", defaults.KafkaRetryAttempts, "Number of retry attempts for Kafka operations")
	flag.Parse()
	return defaults
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
