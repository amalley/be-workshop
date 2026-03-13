package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/AMalley/be-workshop/ch-3/api/controller/wikistats"
	"github.com/AMalley/be-workshop/ch-3/api/database/scylla"
	"github.com/AMalley/be-workshop/ch-3/api/middleware"
	"github.com/AMalley/be-workshop/ch-3/api/server"
	"github.com/AMalley/be-workshop/ch-3/api/stream/wiki"
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
	rtr := http.NewServeMux()
	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(args.LogLevel),
	}))
	syl := scylla.NewScyllaDatabaseAdapter(lgr, mustGetEnv("SCYLLA_HOST"))
	stm := wiki.NewWikiStreamAdapter(lgr, syl, args.URL)
	mdl := middleware.NewMiddlewareRegistry()
	ctl := wikistats.NewWikiStatsController(lgr, syl)
	svr := server.NewServer(lgr, rtr, stm, syl, args.Port)

	// Register middleware
	mdl.Use(PanicRecoverMiddleware(lgr))
	mdl.Use(svr.ContextCancelledMiddleware())

	// Register non-middleware dependent endpoints
	rtr.Handle("GET /liveness", http.HandlerFunc(ctl.Liveness))
	rtr.Handle("GET /readiness", http.HandlerFunc(ctl.Readiness))

	// Register middleware dependent endpoints
	rtr.Handle("GET /stats", mdl.Resolve(http.HandlerFunc(ctl.GetStats)))

	// Start the server
	svr.Start()
}

func PanicRecoverMiddleware(logger *slog.Logger) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("PANIC RECOVER",
						slog.Any("err", err),
						slog.String("stack", string(debug.Stack())),
					)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
