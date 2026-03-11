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
	"time"

	"github.com/AMalley/be-workshop/ch-1/api/database"
	"github.com/AMalley/be-workshop/ch-1/models"
)

var dataTag = []byte("data: ")

type WikiStreamAdapter struct {
	stream io.ReadCloser

	logger   *slog.Logger
	database *database.WikiStatsDB
	client   *http.Client
	url      *url.URL
}

func NewWikiStreamAdapter(logger *slog.Logger, database *database.WikiStatsDB) *WikiStreamAdapter {
	const streamURL = "https://stream.wikimedia.org/v2/stream/recentchange"

	u, err := url.Parse(streamURL)
	if err != nil {
		panic(fmt.Errorf("fatal: unable to parse stream URL: %s: %s", streamURL, err.Error()))
	}

	return &WikiStreamAdapter{
		logger:   logger,
		database: database,
		client:   &http.Client{},
		url:      u,
	}
}

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

func (a *WikiStreamAdapter) Consume(ctx context.Context) error {
	if a.stream == nil {
		return errors.New("Attempting to consume without a connected stream")
	}
	return a.consumeStream(ctx)
}

func (a *WikiStreamAdapter) Close(ctx context.Context) error {
	if a.stream == nil {
		return nil
	}
	return a.stream.Close()
}

func (a *WikiStreamAdapter) consumeStream(ctx context.Context) error {
	const maxCapacity = 1024 * 1024
	const wait = time.Second / 4

	reader := bufio.NewReaderSize(a.stream, maxCapacity)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		chunk, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// We hit the end of the stream, we'll wait a bit then continue.
				time.Sleep(wait)
				continue
			}
			return err
		}

		if err := a.consumeChunk(chunk); err != nil {
			return nil
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
