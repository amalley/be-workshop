package consumer

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/amalley/be-workshop/ch-9/api/stream"
	"github.com/amalley/be-workshop/ch-9/api/stream/wiki"
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

const (
	WorkerCount  = 2
	MaxBatchSize = 100
)

var _ stream.Adapter = &WikiStreamAdapterConsumer{}

type WikiStreamAdapterConsumer struct {
	cfg  *wiki.WikiStreamOptions
	wrks []*worker
}

func NewStreamAdapter(options ...wiki.WikiStreamOption) *WikiStreamAdapterConsumer {
	cfg := wiki.ApplyWikiStreamOptions(wiki.DefaultWikiStreamOptions(), options...)
	cfg.Logger = cfg.Logger.With("src", "WikiStreamAdapterConsumer")
	return &WikiStreamAdapterConsumer{
		cfg:  cfg,
		wrks: make([]*worker, WorkerCount),
	}
}

func (a *WikiStreamAdapterConsumer) Connect(ctx context.Context) error {
	for i := 0; i < len(a.wrks); i++ {
		a.cfg.Logger.Info("connecting worker", "worker_id", i)

		wrk := newWorker(a.cfg, a.cfg.Logger, a.cfg.Metrics, a.cfg.DBWriter, uint8(i))
		if err := wrk.connect(ctx); err != nil {
			return err
		}
		a.wrks[i] = wrk
	}
	return nil
}

func (a *WikiStreamAdapterConsumer) Close(ctx context.Context) error {
	for i, wrk := range a.wrks {
		a.cfg.Logger.Info("closing worker", "worker_id", i)

		if err := wrk.close(); err != nil {
			return err
		}
	}
	return nil
}

func (a *WikiStreamAdapterConsumer) Consume(ctx context.Context) error {
	for i, wrk := range a.wrks {
		go func(id uint8, w *worker) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					err := w.consume(ctx)

					if errors.Is(err, context.Canceled) {
						return
					}

					if err != nil {
						w.logger.Error("worker consumption error, retrying", slog.Any("err", err))
					}

					wait := time.NewTicker(2 * time.Second)
					select {
					case <-ctx.Done():
						wait.Stop()
						return
					case <-wait.C:
					}
				}
			}
		}(uint8(i), wrk)
	}
	return nil
}
