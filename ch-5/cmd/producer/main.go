package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/database"
	"github.com/amalley/be-workshop/ch-5/api/database/scylla"
	"github.com/amalley/be-workshop/ch-5/api/handlers/producer"
	"github.com/amalley/be-workshop/ch-5/api/middleware"
	"github.com/amalley/be-workshop/ch-5/api/server"
	"github.com/amalley/be-workshop/ch-5/api/stream"
	"github.com/amalley/be-workshop/ch-5/api/stream/wiki"
	"github.com/amalley/be-workshop/ch-5/cli"
	"github.com/gocql/gocql"
)

func main() {
	args := cli.ParseArgs(&cli.Args{
		Port:         cli.GetEnv("PORT", "7000"),
		LogLevel:     cli.GetEnv("LOG_LEVEL", "info"),
		URL:          cli.GetEnv("STREAM_URL", "https://stream.wikimedia.org/v2/stream/recentchange"),
		DBHost:       cli.GetEnv("DB_HOST", "scylla"),
		DBKeyspace:   cli.GetEnv("DB_KEYSPACE", "wikistats"),
		KafkaBrokers: cli.GetEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopic:   cli.GetEnv("KAFKA_TOPIC", "wikistats"),
	})

	mux := http.NewServeMux()
	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cli.ParseLogLevel(args.LogLevel),
	})).With("svc", "wikistats-producer")

	syl := scylla.NewScyllaDatabaseAdapter(
		scylla.WithLogger(lgr),
		scylla.WithHost(args.DBHost),
		scylla.WithKeyspace(args.DBKeyspace),
		scylla.WithClusterConsistency(gocql.Quorum),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
	)

	u, err := url.Parse(args.URL)
	if err != nil {
		lgr.Error("fatal: unable to parse stream URL", "url", args.URL, "error", err.Error())
		os.Exit(1)
	}

	stm := wiki.NewWikiStreamAdapter(
		wiki.WithLogger(lgr),
		wiki.WithURL(u),
		wiki.WithTopic(args.KafkaTopic),
		wiki.WithBrokers(strings.Split(args.KafkaBrokers, ",")),
		wiki.WithRetryAttempts(args.KafkaRetryAttempts),
	)

	hld := producer.NewProducerHandlers(lgr)
	hld.RegisterHandlers(mux)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mdl := middleware.NewMiddlewareRegistry()
	mdl.Use(middleware.PanicRecover(lgr))

	server.NewServer(
		server.WithAddress(":"+args.Port),
		server.WithHandler(mdl.Resolve(mux)),
		server.WithLogger(lgr),
		server.WithStartupHook(startup(lgr, syl, stm)),
		server.WithShutdownHook(shutdown(lgr, syl, stm)),
	).Run(ctx)
}

func startup(logger *slog.Logger, dbAdapter database.Adapter, streamAdapter stream.StreamAdapter) func(context.Context) error {
	return func(ctx context.Context) error {
		var grp sync.WaitGroup
		grp.Go(func() {
			if err := dbAdapter.Connect(ctx); err != nil {
				logger.Error("failed to connect to database", "error", err.Error())
				return
			}
		})
		grp.Go(func() {
			if err := streamAdapter.Connect(ctx); err != nil {
				logger.Error("failed to connect to stream", "error", err.Error())
				return
			}
		})
		grp.Wait()

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

		go func() {
			if err := streamAdapter.Consume(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("stream consumption ended with error", "error", err.Error())
			}
		}()

		return nil
	}
}

func shutdown(logger *slog.Logger, dbAdapter database.Adapter, streamAdapter stream.StreamAdapter) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := streamAdapter.Close(ctx); err != nil {
			logger.Error("failed to close stream adapter", "error", err.Error())
		}

		if err := dbAdapter.Close(ctx); err != nil {
			logger.Error("failed to close database adapter", "error", err.Error())
		}
		return nil
	}
}
