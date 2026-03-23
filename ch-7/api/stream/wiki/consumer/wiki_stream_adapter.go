package consumer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/amalley/be-workshop/ch-7/api/metrics"
	"github.com/amalley/be-workshop/ch-7/api/stream"
	"github.com/amalley/be-workshop/ch-7/api/stream/wiki"
	"github.com/amalley/be-workshop/ch-7/api/utils"
	"github.com/amalley/be-workshop/ch-7/models"
	"github.com/amalley/be-workshop/ch-7/models/gen/pbwiki"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

const (
	MetricsWikiRedpandaEventsConsumed                  = "wiki_redpanda_events_consumed"
	MetricsWikiRedpandaEventsConsumedHelp              = "Total number of wiki events consumed from Redpanda"
	MetricsWikiRedpandaEventsProcessedSuccessfully     = "wiki_redpanda_events_processed_successfully"
	MetricsWikiRedpandaEventsProcessedSuccessfullyHelp = "Total number of wiki events processed successfully from Redpanda "
	MetricsWikiRedpandaEventsProcessedFailed           = "wiki_redpanda_events_processed_failed"
	MetricsWikiRedpandaEventsProcessedFailedHelp       = "Total number of wiki eventsthat failed to process from Redpanda "
)

var (
	ErrAlreadyConnected = errors.New("stream adapter is already connected")
	ErrNotConnected     = errors.New("stream adapter is not connected")
)

const MaxBatchSize = 100

var _ stream.Adapter = &WikiStreamAdapterConsumer{}

type WikiStreamAdapterConsumer struct {
	cfg    *wiki.WikiStreamOptions
	client *kgo.Client
}

func NewStreamAdapter(options ...wiki.WikiStreamOption) *WikiStreamAdapterConsumer {
	cfg := wiki.ApplyWikiStreamOptions(wiki.DefaultWikiStreamOptions(), options...)
	cfg.Logger = cfg.Logger.With("src", "WikiStreamAdapterConsumer")
	return &WikiStreamAdapterConsumer{cfg: cfg, client: nil}
}

func (a *WikiStreamAdapterConsumer) Connect(ctx context.Context) error {
	if a.client != nil {
		return ErrAlreadyConnected
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(a.cfg.Brokers...),
		kgo.ConsumeTopics(a.cfg.Topic),
		kgo.ConsumerGroup(a.cfg.ConsumerGroupID),
		kgo.FetchMaxWait(a.cfg.FetchMaxWait),
		kgo.FetchMinBytes(a.cfg.FetchMinBytes),
	)
	if err != nil {
		return err
	}
	a.client = client

	return nil
}

func (a *WikiStreamAdapterConsumer) Close(ctx context.Context) error {
	if a.client == nil {
		return nil
	}

	a.client.Close()
	a.client = nil

	return nil
}

func (a *WikiStreamAdapterConsumer) Consume(ctx context.Context) error {
	if a.client == nil {
		return ErrNotConnected
	}

	for {
		// Check if the context is done before polling for records to allow for graceful shutdown.
		if utils.CtxDone(ctx) {
			return ctx.Err()
		}

		fetches := a.client.PollRecords(ctx, a.cfg.MaxPollRecords)
		if fetches.IsClientClosed() {
			return ErrNotConnected
		}

		// Check if the context is done after polling for records to allow for graceful shutdown.
		// This is important in case the ctx was canceled while waiting for the records to be fetched.
		if utils.CtxDone(ctx) {
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			a.cfg.Logger.Error("error fetching records", slog.Any("errs", errs))
		}

		records := make([]*kgo.Record, 0, a.cfg.MaxPollRecords)
		fetches.EachRecord(func(record *kgo.Record) {
			records = append(records, record)
		})

		a.recordMetrics(MetricsWikiRedpandaEventsConsumed, float64(len(records)))

		batches := utils.BatchSlice(records, MaxBatchSize)
		totals, success, failed, err := a.handleBatches(ctx, batches)

		if err != nil {
			a.cfg.Logger.Error("error handling batches", slog.Any("err", err))
		}

		a.recordMetrics(MetricsWikiRedpandaEventsProcessedSuccessfully, float64(success))
		a.recordMetrics(MetricsWikiRedpandaEventsProcessedFailed, float64(failed))

		if a.cfg.DBWriter == nil {
			a.cfg.Logger.Info("batch processed", slog.Any("totals", totals))
			continue
		}

		if err := a.cfg.DBWriter.InsertStats(ctx, totals); err != nil {
			a.cfg.Logger.Error("error inserting stats to database", slog.Any("err", err))
		}
	}
}

func (a *WikiStreamAdapterConsumer) handleBatches(ctx context.Context, batches [][]*kgo.Record) (*models.WikiStatsCounts, int64, int64, error) {
	var grp sync.WaitGroup
	grp.Add(len(batches))

	errch := make(chan error, len(batches))
	countch := make(chan *models.WikiStatsCounts, len(batches))

	var successCount atomic.Int64
	var failedCount atomic.Int64

	// Process each batch in a separate goroutine to allow for concurrent processing of records.
	for i, batch := range batches {
		go func(batch []*kgo.Record, batchNum int) {
			defer grp.Done()

			if utils.CtxDone(ctx) {
				return
			}

			count, success, failed, err := a.handleBatch(ctx, batch)
			if err != nil {
				a.cfg.Logger.Error("error handling batch",
					slog.Any("err", err),
					slog.Int("batch_num", batchNum),
				)
				errch <- err
			}
			if count != nil {
				countch <- count
			}
			successCount.Add(int64(success))
			failedCount.Add(int64(failed))
		}(batch, i)
	}
	grp.Wait()

	close(errch)
	close(countch)

	if utils.CtxDone(ctx) {
		return nil, successCount.Load(), failedCount.Load(), ctx.Err()
	}

	var errs []error
	for err := range errch {
		errs = append(errs, err)
	}

	var totals models.WikiStatsCounts
	for count := range countch {
		totals.Messages += count.Messages
		totals.Users += count.Users
		totals.Bots += count.Bots
		totals.Servers += count.Servers
	}

	if len(errs) > 0 {
		return &totals, successCount.Load(), failedCount.Load(), errors.Join(errs...)
	}
	return &totals, successCount.Load(), failedCount.Load(), nil
}

func (a *WikiStreamAdapterConsumer) handleBatch(ctx context.Context, batch []*kgo.Record) (*models.WikiStatsCounts, int64, int64, error) {
	var success int64
	var failed int64

	if utils.CtxDone(ctx) {
		return nil, success, failed, ctx.Err()
	}

	errs := make([]error, 0, len(batch))

	var counts models.WikiStatsCounts
	for _, record := range batch {
		var message pbwiki.WikiStreamMessage

		if err := proto.Unmarshal(record.Value, &message); err != nil {
			a.cfg.Logger.Error("error unmarshaling record", slog.Any("err", err), slog.String("raw", string(record.Value)))
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

func (a *WikiStreamAdapterConsumer) recordMetrics(id string, count float64) {
	if a.cfg.Metrics != nil {
		if err := a.cfg.Metrics.Increment(metrics.RecorderID(id), count); err != nil {
			a.cfg.Logger.Error("failed to record metrics", slog.Any("err", err), slog.String("metric", id))
		}
	}
}
