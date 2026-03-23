package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/amalley/be-workshop/ch-8/api/authentication/public"
	"github.com/amalley/be-workshop/ch-8/api/database"
	"github.com/amalley/be-workshop/ch-8/api/database/scylla"
	"github.com/amalley/be-workshop/ch-8/api/handlers/consumer"
	wikiprom "github.com/amalley/be-workshop/ch-8/api/metrics/prometheus"
	"github.com/amalley/be-workshop/ch-8/api/middleware"
	"github.com/amalley/be-workshop/ch-8/api/server"
	"github.com/amalley/be-workshop/ch-8/api/stream"
	"github.com/amalley/be-workshop/ch-8/api/stream/wiki"
	wikicons "github.com/amalley/be-workshop/ch-8/api/stream/wiki/consumer"
	"github.com/amalley/be-workshop/ch-8/api/utils"
	"github.com/amalley/be-workshop/ch-8/cli"
	"github.com/gocql/gocql"
)

func main() {
	args := cli.ParseArgs(&cli.Args{
		Port:                 utils.MustParseInt(cli.GetEnv("PORT", "7000")),
		LogLevel:             cli.GetEnv("LOG_LEVEL", "debug"),
		DBHost:               cli.GetEnv("DB_HOST", "scylla"),
		DBKeyspace:           cli.GetEnv("DB_KEYSPACE", "wikistats"),
		KafkaBrokers:         cli.GetEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopic:           cli.GetEnv("KAFKA_TOPIC", "wikistats"),
		KafkaRetryAttempts:   utils.MustParseInt(cli.GetEnv("KAFKA_RETRY_ATTEMPTS", "5")),
		KafkaMaxPollRecords:  utils.MustParseInt(cli.GetEnv("KAFKA_MAX_POLL_RECORDS", "1000")),
		KafkaConsumerGroupID: cli.GetEnv("KAFKA_CONSUMER_GROUP_ID", "wikistats-consumers"),
	})

	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cli.ParseLogLevel(args.LogLevel),
	})).With("svc", "wikistats-consumer")

	syl := scylla.NewScyllaDatabaseAdapter(
		scylla.WithLogger(lgr),
		scylla.WithHost(args.DBHost),
		scylla.WithKeyspace(args.DBKeyspace),
		scylla.WithClusterConsistency(gocql.Quorum),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
	)

	mtx := wikiprom.NewPrometheusAdapter(lgr)
	mtx.AddRecorders(
		wikiprom.NewPrometheusCounter(
			wikicons.MetricsWikiRedpandaEventsConsumed,
			wikicons.MetricsWikiRedpandaEventsConsumedHelp,
			mtx.Registry(),
		),
		wikiprom.NewPrometheusCounter(
			wikicons.MetricsWikiRedpandaEventsProcessedSuccessfully,
			wikicons.MetricsWikiRedpandaEventsProcessedSuccessfullyHelp,
			mtx.Registry(),
		),
		wikiprom.NewPrometheusCounter(
			wikicons.MetricsWikiRedpandaEventsProcessedFailed,
			wikicons.MetricsWikiRedpandaEventsProcessedFailedHelp,
			mtx.Registry(),
		),
	)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", http.HandlerFunc(mtx.HttpHandler))

	pub := public.NewPublicAuthenticator(lgr)

	hdl := consumer.NewConsumerHandlers(lgr, syl, pub)
	hdl.RegisterHandlers(mux)

	adp := wikicons.NewStreamAdapter(
		wiki.WithLogger(lgr),
		wiki.WithBrokers(strings.Split(args.KafkaBrokers, ",")),
		wiki.WithTopic(args.KafkaTopic),
		wiki.WithConsumerGroupID(args.KafkaConsumerGroupID),
		wiki.WithRetryAttempts(args.KafkaRetryAttempts),
		wiki.WithFetchMaxWait(500*time.Millisecond),
		wiki.WithMaxPollRecords(args.KafkaMaxPollRecords),
		wiki.WithDBWriter(syl),
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
		server.WithStartupHook(startup(lgr, syl, adp)),
		server.WithShutdownHook(shutdown(lgr, syl, adp)),
	).Run(ctx)
}

func startup(logger *slog.Logger, dbAdapter database.Adapter, streamAdapter stream.Adapter) server.ServerHook {
	return func(ctx context.Context) error {
		if utils.CtxDone(ctx) {
			logger.Info("failed to start", slog.String("reason", ctx.Err().Error()))
			return ctx.Err()
		}

		// Blocks until services are connected.
		// If the context is canceled, the function will return early with the cancelation error.
		if err := connectServices(ctx, logger, dbAdapter, streamAdapter); err != nil {
			logger.Error("startup failed", slog.Any("err", err))
			return err
		}

		// Blocks until the database is ready.
		// If the context is canceled, the function will return early with the cancelation error.
		if err := waitForDatabase(ctx, logger, dbAdapter); err != nil {
			logger.Error("failed while waiting for database to be ready", slog.Any("err", err))
			return err
		}

		if err := streamAdapter.Consume(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("stream consumption ended with error", slog.Any("err", err))
		}

		return nil
	}
}

func shutdown(logger *slog.Logger, dbAdapter database.Adapter, streamAdapter stream.Adapter) server.ServerHook {
	return func(ctx context.Context) error {
		if utils.CtxDone(ctx) {
			logger.Info("failed to shutdown", slog.String("reason", ctx.Err().Error()))
			return ctx.Err()
		}

		if err := streamAdapter.Close(ctx); err != nil {
			logger.Error("failed to close stream adapter", slog.Any("err", err))
		}

		if err := dbAdapter.Close(ctx); err != nil {
			logger.Error("failed to close database", slog.Any("err", err))
		}

		return nil
	}
}

func connectServices(ctx context.Context, logger *slog.Logger, dbAdapter database.Adapter, streamAdapter stream.Adapter) error {
	errCh := make(chan error, 2)

	var grp sync.WaitGroup
	grp.Go(func() {
		if err := streamAdapter.Connect(ctx); err != nil {
			logger.Error("failed to connect to stream adapter", slog.Any("err", err))
			errCh <- err
		}
	})
	grp.Go(func() {
		if err := dbAdapter.Connect(ctx); err != nil {
			logger.Error("failed to connect to database", slog.Any("err", err))
			errCh <- err
		}
	})
	grp.Wait()

	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func waitForDatabase(ctx context.Context, logger *slog.Logger, dbAdapter database.Adapter) error {
	wait := time.NewTicker(time.Second * 2)
	defer wait.Stop()

	for !dbAdapter.IsReady() {
		logger.Info("Waiting for database to be ready...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait.C:
		}
	}

	_, exists, err := dbAdapter.GetUser(ctx, "admin")
	if err != nil {
		logger.Error("Failed to get default user", slog.Any("err", err))
	}

	if !exists {
		// Don't do this in production
		if err := dbAdapter.CreateUser(ctx, "admin", "password"); err != nil {
			logger.Error("Failed to create default user", slog.Any("err", err))
		}
	}
	return nil
}
