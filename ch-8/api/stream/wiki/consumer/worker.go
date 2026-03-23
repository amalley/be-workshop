package consumer

import (
	"context"
	"errors"
	"log/slog"

	"github.com/amalley/be-workshop/ch-8/api/database"
	"github.com/amalley/be-workshop/ch-8/api/metrics"
	"github.com/amalley/be-workshop/ch-8/api/stream/wiki"
	"github.com/amalley/be-workshop/ch-8/api/utils"
	"github.com/amalley/be-workshop/ch-8/models"
	"github.com/amalley/be-workshop/ch-8/models/gen/pbwiki"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

type worker struct {
	logger *slog.Logger
	client *kgo.Client
	cfg    *wiki.WikiStreamOptions

	metrics  metrics.Adapter
	database database.Writer
}

func newWorker(
	cfg *wiki.WikiStreamOptions,
	logger *slog.Logger,
	metrics metrics.Adapter,
	database database.Writer,
	id uint8) *worker {
	return &worker{
		logger:   logger.With("worker_id", id),
		cfg:      cfg,
		metrics:  metrics,
		database: database,
	}
}

func (w *worker) connect(ctx context.Context) error {
	if w.client != nil {
		return ErrAlreadyConnected
	}

	if w.database == nil {
		return errors.New("database writer is not configured")
	}

	opts := []kgo.Opt{
		kgo.WithContext(ctx),
		kgo.SeedBrokers(w.cfg.Brokers...),
		kgo.ConsumeTopics(w.cfg.Topic),
		kgo.ConsumerGroup(w.cfg.ConsumerGroupID),
		kgo.FetchMaxWait(w.cfg.FetchMaxWait),
		kgo.FetchMinBytes(w.cfg.FetchMinBytes),
		kgo.DisableAutoCommit(),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	w.client = client

	return nil
}

func (w *worker) close() error {
	if w.client == nil {
		return ErrNotConnected
	}

	w.client.Close()
	w.client = nil
	return nil
}

func (w *worker) consume(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			fetches := w.client.PollRecords(ctx, w.cfg.MaxPollRecords)
			if fetches.IsClientClosed() {
				return ErrNotConnected
			}

			if utils.CtxDone(ctx) {
				return ctx.Err()
			}

			if errs := fetches.Errors(); len(errs) > 0 {
				w.logger.Error("error fetching records", slog.Any("errs", errs))
				e := make([]error, len(errs))
				for i, fe := range errs {
					e[i] = fe.Err
				}
				return errors.Join(e...)
			}

			records := make([]*kgo.Record, 0, w.cfg.MaxPollRecords)
			fetches.EachRecord(func(record *kgo.Record) {
				records = append(records, record)
			})

			// Record the number of events consumed for monitoring purposes.
			w.recordMetrics(MetricsWikiRedpandaEventsConsumed, float64(len(records)))

			totals, success, failed, err := w.handleBatch(ctx, records)
			if err != nil {
				w.logger.Error("error handling batches", slog.Any("err", err))
			}

			// Record the number of failed events for monitoring purposes.
			w.recordMetrics(MetricsWikiRedpandaEventsProcessedFailed, float64(failed))

			if err := w.database.InsertStats(ctx, totals); err != nil {
				w.logger.Error("error inserting stats to database", slog.Any("err", err))
				return err
			}

			// Record the number of successfully processed events for monitoring purposes.
			w.recordMetrics(MetricsWikiRedpandaEventsProcessedSuccessfully, float64(success))

			return w.client.CommitRecords(ctx, records...)
		}
	}
}

func (w *worker) handleBatch(ctx context.Context, batch []*kgo.Record) (*models.WikiStatsCounts, int64, int64, error) {
	var success int64
	var failed int64

	if utils.CtxDone(ctx) {
		return nil, 0, 0, ctx.Err()
	}

	errs := make([]error, 0, len(batch))

	var counts models.WikiStatsCounts
	for _, record := range batch {
		var message pbwiki.WikiStreamMessage

		if err := proto.Unmarshal(record.Value, &message); err != nil {
			w.cfg.Logger.Error("error unmarshaling record", slog.Any("err", err), slog.String("raw", string(record.Value)))
			errs = append(errs, err)
			failed++
			continue
		}

		if message.Meta.Id != nil {
			counts.Messages++
		}
		if message.User != "" {
			counts.Users++
		}
		if message.Bot {
			counts.Bots++
		}
		if message.ServerName != "" {
			counts.Servers++
		}
		success++
	}

	// Ensure the context is not done before attempting to insert stats to allow for graceful shutdown.
	if utils.CtxDone(ctx) {
		return nil, success, failed, ctx.Err()
	}

	if len(errs) > 0 {
		return &counts, success, failed, errors.Join(errs...)
	}

	return &counts, success, failed, nil
}

func (w *worker) recordMetrics(id string, count float64) {
	if w.metrics == nil || count == 0 {
		return
	}
	if err := w.metrics.Increment(metrics.RecorderID(id), count); err != nil {
		w.logger.Error("failed to record metrics", slog.Any("err", err), slog.String("metric", id))
	}
}
