package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/amalley/be-workshop/ch-1/api/database"
	"github.com/amalley/be-workshop/ch-1/models"
)

var dataTag = []byte("data: ")

// WikiStreamRequestDoer defines the interface of a http request doer - often http.Client
type WikiStreamRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type WikiStreamAdapter struct {
	stream io.ReadCloser
	client WikiStreamRequestDoer

	logger   *slog.Logger
	database *database.WikiStatsDB
	url      *url.URL
}

// NewWikiStreamAdapter returns a new Wiki stream adapter using http.DefaultClient as the underlying request doer.
func NewWikiStreamAdapter(logger *slog.Logger, database *database.WikiStatsDB) *WikiStreamAdapter {
	return NewWikiStreamAdapterWithClient(logger, database, http.DefaultClient)
}

// NewWikiStreamAdapter returns a new Wiki stream adapter using the providered client as the underlying requeust doer.
func NewWikiStreamAdapterWithClient(logger *slog.Logger, database *database.WikiStatsDB, client WikiStreamRequestDoer) *WikiStreamAdapter {
	const streamURL = "https://stream.wikimedia.org/v2/stream/recentchange"

	u, err := url.Parse(streamURL)
	if err != nil {
		panic(fmt.Errorf("fatal: unable to parse stream URL: %s: %s", streamURL, err.Error()))
	}

	return &WikiStreamAdapter{
		logger:   logger,
		database: database,
		client:   client,
		url:      u,
	}
}

// Connect established a connection to the Wiki media stream
func (a *WikiStreamAdapter) Connect(ctx context.Context) error {
	a.logger.Info("Connecting to WikiMedia stream...")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "REDspace workshop (aaron.malley@redspace.com)")
	req.Header.Set("Content-type", "application/json")

	rsp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	a.stream = rsp.Body

	a.logger.Info("...Successfully connected to WikiMedia stream")
	return nil
}

// Consume processes the Wiki media stream, consuming messages containing the "data: " tag.
// Consumed messages are added to the provided WikiStats database given to the adapter on initialization.
//
// Returns an error if attempting to consume before connecting to a stream.
func (a *WikiStreamAdapter) Consume(ctx context.Context) error {
	if a.stream == nil {
		return errors.New("Attempting to consume without a connected stream")
	}
	return a.consumeStream(ctx)
}

// Close ensure the stream if closed. Does nothing if the adapter is not connected to a stream.
func (a *WikiStreamAdapter) Close(ctx context.Context) error {
	if a.stream == nil {
		return nil
	}
	return a.stream.Close()
}

func (a *WikiStreamAdapter) consumeStream(ctx context.Context) error {
	const maxCapacity = 1024 * 1024

	reader := bufio.NewReaderSize(a.stream, maxCapacity)

	for {
		// Ensure there hasn't been a context error before continuing
		if err := a.ctxErr(ctx, nil); err != nil {
			return err
		}

		chunk, err := reader.ReadBytes('\n')
		if cerr := a.ctxErr(ctx, err); cerr != nil {
			return cerr
		}

		if err := a.consumeChunk(chunk); err != nil {
			return a.ctxErr(ctx, err)
		}
	}
}

func (a *WikiStreamAdapter) consumeChunk(chunk []byte) error {
	if !bytes.HasPrefix(chunk, dataTag) {
		return nil // Skip non-data chunks
	}

	var message models.WikiStreamMessage

	raw := chunk[len(dataTag):]
	if err := json.Unmarshal(raw, &message); err != nil {
		a.logger.Error("Failed to unmarshal message", slog.Any("raw", string(raw)), slog.Any("err", err))
		return err
	}

	a.database.Insert(message.Meta.ID, message.User, message.ServerName, message.Bot)
	return nil
}

// ctxErr checks if the context is done before returning the provided err.
// If the context is done, then the its err is returned instead.
func (a *WikiStreamAdapter) ctxErr(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return err
	}
}
