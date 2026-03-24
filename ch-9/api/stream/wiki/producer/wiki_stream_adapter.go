package producer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/amalley/be-workshop/ch-9/api/metrics"
	"github.com/amalley/be-workshop/ch-9/api/stream"
	"github.com/amalley/be-workshop/ch-9/api/stream/wiki"
	"github.com/amalley/be-workshop/ch-9/models/gen/pbwiki"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	MetricsWikiStreamEventsConsumed     = "wiki_stream_events_consumed"
	MetricsWikiStreamEventsConsumedHelp = "Total number of wiki events consumed from the stream"
	MetricsWikiRecordsProduced          = "wiki_records_produced"
	MetricsWikiRecordsProducedHelp      = "Total number of wiki records produced to Kafka"
)

var (
	idTag   = []byte("id:")
	dataTag = []byte("data:")

	protojsonUnmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	ErrNotConnected     = errors.New("not connected to stream")
	ErrAlreadyConnected = errors.New("already connected to stream")
)

var _ stream.Adapter = &WikiStreamAdapterProducer{}

type WikiStreamAdapterProducer struct {
	cfg    *wiki.WikiStreamOptions
	client *kgo.Client

	LastEventID string
}

// NewStreamAdapter returns a new Wiki stream adapter using http.DefaultClient as the underlying request doer.
func NewStreamAdapter(options ...wiki.WikiStreamOption) *WikiStreamAdapterProducer {
	return NewWikiStreamAdapterWithClient(options...)
}

// NewWikiStreamAdapterWithClient returns a new Wiki stream adapter using the provided client as the underlying request doer.
func NewWikiStreamAdapterWithClient(options ...wiki.WikiStreamOption) *WikiStreamAdapterProducer {
	cfg := wiki.ApplyWikiStreamOptions(wiki.DefaultWikiStreamOptions(), options...)
	cfg.Logger = cfg.Logger.With("src", "WikiStreamAdapterProducer")
	return &WikiStreamAdapterProducer{cfg: cfg, client: nil}
}

// Connect established a connection to the Wiki media stream
func (a *WikiStreamAdapterProducer) Connect(ctx context.Context) error {
	if a.client != nil {
		return ErrAlreadyConnected
	}

	opt := []kgo.Opt{
		kgo.SeedBrokers(a.cfg.Brokers...),
		kgo.DefaultProduceTopic(a.cfg.Topic),
		kgo.RecordRetries(a.cfg.RetryAttempts),
		kgo.MetadataMaxAge(5 * time.Second),
	}
	if a.cfg.AutoTopicCreation {
		opt = append(opt, kgo.AllowAutoTopicCreation())
	}

	client, err := kgo.NewClient(opt...)
	if err != nil {
		return err
	}
	a.client = client

	return nil
}

// Close ensure the stream if closed. Does nothing if the adapter is not connected to a stream.
func (a *WikiStreamAdapterProducer) Close(ctx context.Context) error {
	if a.client == nil {
		return nil
	}

	a.client.Close()
	a.client = nil

	return nil
}

// Consume starts consuming the stream and producing messages to Kafka.
// If the connection to the stream is lost, it will attempt to reconnect with an exponential backoff strategy.
// The context can be used to signal the adapter to stop consuming and close the connection to the stream.
func (a *WikiStreamAdapterProducer) Consume(ctx context.Context) error {
	if a.client == nil {
		return ErrNotConnected
	}

	a.cfg.Logger.Info("connecting to wiki stream", slog.String("url", a.cfg.URL.String()))
	backoff := time.Second

	defer func() {
		if err := a.client.Flush(ctx); err != nil {
			a.cfg.Logger.Error("failed to flush Kafka producer", slog.Any("err", err))
		}
		a.cfg.Logger.Info("stopped consuming wiki stream")
	}()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Attempt to open and read the stream. If the connection is lost, attempt to reconnect.
		stream, err := a.openStream(ctx)
		if err == nil {
			a.cfg.Logger.Info("connected to wiki stream",
				slog.String("url", a.cfg.URL.String()),
				slog.String("last_event_id", a.LastEventID),
			)
			backoff = time.Second

			err = a.readStream(ctx, stream)
			if err := stream.Close(); err != nil {
				a.cfg.Logger.Error("failed to close stream", slog.Any("err", err))
			}
			if err != nil {
				a.cfg.Logger.Error("error reading from stream", slog.Any("err", err))
			}
		}

		// If the context was canceled, return the error to allow for graceful shutdown.
		// Otherwise, log the error and attempt to reconnect.
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return ctx.Err()
		}

		a.cfg.Logger.Error("stream connection lost",
			slog.Any("err", err),
			slog.Float64("retry_in", backoff.Seconds()),
		)

		// Connect is lost, wait a bit and try again.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff = min(backoff*2, time.Minute)
		}
	}
}

func (a *WikiStreamAdapterProducer) Client() *kgo.Client {
	return a.client
}

func (a *WikiStreamAdapterProducer) readStream(ctx context.Context, stream io.ReadCloser) error {
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	reader := bufio.NewReaderSize(stream, maxLineSize)

	// Read the stream line by line. Lines starting with "id:" are treated as event IDs and stored for reconnection.
	// Lines starting with "data:" are treated as messages and produced to Kafka.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if after, ok := bytes.CutPrefix(line, idTag); ok {
			a.LastEventID = string(bytes.TrimSpace(after))
			continue
		}

		if after, ok := bytes.CutPrefix(line, dataTag); ok {
			after = bytes.TrimSpace(after)

			var message pbwiki.WikiStreamMessage
			if err := protojsonUnmarshalOptions.Unmarshal(after, &message); err != nil {
				a.cfg.Logger.Error("error unmarshaling message", slog.Any("err", err), slog.String("raw", string(after)))
				continue
			}

			value, err := proto.Marshal(&message)
			if err != nil {
				a.cfg.Logger.Error("error marshaling message", slog.Any("err", err), slog.String("raw", string(after)))
				continue
			}

			record := &kgo.Record{
				Topic: a.cfg.Topic,
				Value: value,
			}

			a.recordMetrics(MetricsWikiStreamEventsConsumed, 1)
			a.client.Produce(ctx, record, func(r *kgo.Record, err error) {
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					a.cfg.Logger.Error("failed to produce message to Kafka", slog.Any("err", err), slog.String("raw", string(after)))
					return
				}
				a.recordMetrics(MetricsWikiRecordsProduced, 1)
			})
		}
	}
}

func (a *WikiStreamAdapterProducer) openStream(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", err.Error())
	}

	req.Header.Set("User-Agent", "REDspace workshop (aaron.malley@redspace.com)")
	if a.LastEventID != "" {
		req.Header.Set("Last-Event-ID", a.LastEventID)
	}

	resp, err := a.cfg.Doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %s", err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return nil, fmt.Errorf("failed to close response body: %s", err.Error())
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (a *WikiStreamAdapterProducer) recordMetrics(id string, count float64) {
	if a.cfg.Metrics != nil {
		if err := a.cfg.Metrics.Increment(metrics.RecorderID(id), count); err != nil {
			a.cfg.Logger.Error("failed to record metrics", slog.Any("err", err), slog.String("metric", id))
		}
	}
}
