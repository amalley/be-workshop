package wiki

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
	netUrl "net/url"

	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/api/utils"
	"github.com/AMalley/be-workshop/ch-3/models"
)

var dataTag = []byte("data: ")

// WikiStreamRequestDoer defines the interface of a http request doer - often http.Client
type WikiStreamRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type WikiStreamAdapter struct {
	stream io.ReadCloser
	client WikiStreamRequestDoer

	database database.DatabaseAdpater

	logger *slog.Logger
	url    *url.URL
}

// NewWikiStreamAdapter returns a new Wiki stream adapter using http.DefaultClient as the underlying request doer.
func NewWikiStreamAdapter(logger *slog.Logger, database database.DatabaseAdpater, url string) *WikiStreamAdapter {
	return NewWikiStreamAdapterWithClient(logger, database, http.DefaultClient, url)
}

// NewWikiStreamAdapter returns a new Wiki stream adapter using the providered client as the underlying requeust doer.
func NewWikiStreamAdapterWithClient(logger *slog.Logger, database database.DatabaseAdpater, client WikiStreamRequestDoer, url string) *WikiStreamAdapter {
	u, err := netUrl.Parse(url)
	if err != nil {
		panic(fmt.Errorf("fatal: unable to parse stream URL: %s: %s", url, err.Error()))
	}

	return &WikiStreamAdapter{
		logger:   logger.With(slog.String("src", "WikiStreamAdapter")),
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
		if err := utils.CtxErr(ctx, nil); err != nil {
			return err
		}

		chunk, err := reader.ReadBytes('\n')
		if cerr := utils.CtxErr(ctx, err); cerr != nil {
			return cerr
		}

		if err := a.consumeChunk(chunk); err != nil {
			return utils.CtxErr(ctx, err)
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

	return a.database.InsertStats(models.WikiStatsModel{
		Message: message.Meta.ID,
		User:    message.User,
		Server:  message.ServerName,
		IsBot:   message.Bot,
	})
}
