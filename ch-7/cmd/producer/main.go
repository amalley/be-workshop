package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/amalley/be-workshop/ch-7/api/handlers/producer"
	wikiprom "github.com/amalley/be-workshop/ch-7/api/metrics/prometheus"
	"github.com/amalley/be-workshop/ch-7/api/middleware"
	"github.com/amalley/be-workshop/ch-7/api/server"
	"github.com/amalley/be-workshop/ch-7/api/stream"
	"github.com/amalley/be-workshop/ch-7/api/stream/wiki"
	wikiprod "github.com/amalley/be-workshop/ch-7/api/stream/wiki/producer"
	"github.com/amalley/be-workshop/ch-7/api/utils"
	"github.com/amalley/be-workshop/ch-7/cli"
)

func main() {
	args := cli.ParseArgs(&cli.Args{
		Port:         utils.MustParseInt(cli.GetEnv("PORT", "7000")),
		LogLevel:     cli.GetEnv("LOG_LEVEL", "info"),
		URL:          cli.GetEnv("STREAM_URL", "https://stream.wikimedia.org/v2/stream/recentchange"),
		KafkaBrokers: cli.GetEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopic:   cli.GetEnv("KAFKA_TOPIC", "wikistats"),
	})

	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cli.ParseLogLevel(args.LogLevel),
	})).With("svc", "wikistats-producer")

	u, err := url.Parse(args.URL)
	if err != nil {
		lgr.Error("fatal: unable to parse stream URL", "url", args.URL, "error", err.Error())
		os.Exit(1)
	}

	mtx := wikiprom.NewPrometheusAdapter(lgr)
	mtx.AddRecorders(
		wikiprom.NewPrometheusCounter(
			wikiprod.MetricsWikiStreamEventsConsumed,
			wikiprod.MetricsWikiStreamEventsConsumedHelp,
			mtx.Registry(),
		),
		wikiprom.NewPrometheusCounter(
			wikiprod.MetricsWikiRecordsProduced,
			wikiprod.MetricsWikiRecordsProducedHelp,
			mtx.Registry(),
		),
	)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", http.HandlerFunc(mtx.HttpHandler))

	hld := producer.NewProducerHandlers(lgr)
	hld.RegisterHandlers(mux)

	adp := wikiprod.NewStreamAdapter(
		wiki.WithLogger(lgr),
		wiki.WithURL(u),
		wiki.WithTopic(args.KafkaTopic),
		wiki.WithBrokers(strings.Split(args.KafkaBrokers, ",")),
		wiki.WithRetryAttempts(args.KafkaRetryAttempts),
		wiki.WithMetrics(mtx),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(middleware.PanicRecover(lgr))

	server.NewServer(
		server.WithAddress(":"+strconv.Itoa(args.Port)),
		server.WithHandler(mdl.Resolve(mux)),
		server.WithLogger(lgr),
		server.WithStartupHook(startup(lgr, adp)),
		server.WithShutdownHook(shutdown(lgr, adp)),
	).Run(ctx)
}

func startup(logger *slog.Logger, streamAdapter stream.Adapter) func(context.Context) error {
	return func(ctx context.Context) error {
		if utils.CtxDone(ctx) {
			logger.Info("failed to start", slog.Any("reason", ctx.Err().Error()))
			return ctx.Err()
		}

		// Blocks until the stream adapter is connected.
		// If the context is canceled, the function will return early with the cancelation error.
		if err := streamAdapter.Connect(ctx); err != nil {
			logger.Error("failed to connect to stream adapter", slog.Any("err", err))
			return err
		}

		// Start consuming in a separate goroutine to avoid blocking the startup process.
		// The stream adapter will handle reconnection logic internally, so we don't need to worry about that here.
		go func() {
			if err := streamAdapter.Consume(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("stream consumption ended with error", "error", err.Error())
			}
		}()

		return nil
	}
}

func shutdown(logger *slog.Logger, streamAdapter stream.Adapter) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := streamAdapter.Close(ctx); err != nil {
			logger.Error("failed to close stream adapter", "error", err.Error())
		}
		return nil
	}
}
