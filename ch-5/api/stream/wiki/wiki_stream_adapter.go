package wiki

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/stream"
	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	idTag   = []byte("id:")
	dataTag = []byte("data:")

	ErrNotConnected     = errors.New("not connected to stream")
	ErrAlreadyConnected = errors.New("already connected to stream")
	ErrAlreadyConsuming = errors.New("already consuming stream")
)

var _ stream.StreamAdapter = &WikiStreamAdapter{}

type WikiStreamAdapter struct {
	cfg    *WikiStreamOptions
	client *kgo.Client

	LastEventID string
}

// NewWikiStreamAdapter returns a new Wiki stream adapter using http.DefaultClient as the underlying request doer.
func NewWikiStreamAdapter(options ...WikiStreamOption) *WikiStreamAdapter {
	return NewWikiStreamAdapterWithClient(options...)
}

// NewWikiStreamAdapterWithClient returns a new Wiki stream adapter using the provided client as the underlying request doer.
func NewWikiStreamAdapterWithClient(options ...WikiStreamOption) *WikiStreamAdapter {
	cfg := applyWikiStreamOptions(defaultWikiStreamOptions(), options...)
	return &WikiStreamAdapter{cfg: cfg, client: nil}
}

// Connect established a connection to the Wiki media stream
func (a *WikiStreamAdapter) Connect(ctx context.Context) error {
	if a.client != nil {
		return ErrAlreadyConnected
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(a.cfg.Brokers...),
		kgo.DefaultProduceTopic(a.cfg.Topic),
		kgo.RecordRetries(a.cfg.RetryAttempts),
		kgo.AllowAutoTopicCreation(),
		kgo.MetadataMaxAge(5*time.Second),
	)
	if err != nil {
		return err
	}
	a.client = client

	return nil
}

// Consume processes the Wiki media stream, consuming messages containing the "data: " tag.
// Consumed messages are added to the provided WikiStats database given to the adapter on initialization.
//
// Returns an error if attempting to consume before connecting to a stream.
func (a *WikiStreamAdapter) Consume(ctx context.Context) error {
	if a.client == nil {
		return ErrNotConnected
	}

	backoff := time.Second

	a.cfg.Logger.Info("connecting to wiki stream", slog.Any("url", a.cfg.URL))
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := a.readStream(ctx)

		if errors.Is(err, context.Canceled) {
			return nil
		}

		a.cfg.Logger.Error("stream connection lost",
			slog.Any("err", err),
			slog.Float64("retry_in", backoff.Seconds()),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff = min(backoff*2, time.Minute)
		}
	}
}

func (a *WikiStreamAdapter) readStream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.URL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %s", err.Error())
	}

	req.Header.Set("User-Agent", "REDspace workshop (aaron.malley@redspace.com)")
	if a.LastEventID != "" {
		req.Header.Set("Last-Event-ID", a.LastEventID)
	}

	resp, err := a.cfg.Doer.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	const maxLineSize = 10 * 1024 * 1024 // 10MB

	reader := bufio.NewReaderSize(resp.Body, maxLineSize)
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
			trimmed := bytes.TrimSpace(after)

			value := make([]byte, len(trimmed))
			copy(value, trimmed)

			record := &kgo.Record{
				Topic: a.cfg.Topic,
				Value: value,
			}

			a.client.Produce(ctx, record, func(r *kgo.Record, err error) {
				if err != nil {
					a.cfg.Logger.Error("failed to produce message to Kafka", slog.Any("err", err))
					return
				}
			})
		}
	}
}

// Close ensure the stream if closed. Does nothing if the adapter is not connected to a stream.
func (a *WikiStreamAdapter) Close(ctx context.Context) error {
	if a.client == nil {
		return nil
	}

	a.client.Close()
	a.client = nil

	return nil
}
