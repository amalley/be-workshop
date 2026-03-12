package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/AMalley/be-workshop/ch-3/api/database"
	"github.com/AMalley/be-workshop/ch-3/models"
)

var ErrMockConnectErr = errors.New("Mock Connect Err")

type MockClient struct {
}

func (c *MockClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
	}, nil
}

type MockErrorClient struct {
}

func (c *MockErrorClient) Do(req *http.Request) (*http.Response, error) {
	return nil, ErrMockConnectErr
}

type MockBufferClient struct {
	buffer io.ReadCloser
}

func (c *MockBufferClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       c.buffer,
	}, nil
}

func TestWikiStreamAdapterConnect(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(t.Output(), nil))
	db := database.NewWikiStatsDB()
	ctx := context.Background()

	adapter := NewWikiStreamAdapterWithClient(logger, db, &MockClient{}, "https://example.com")
	if err := adapter.Connect(ctx); err != nil {
		t.Errorf("Expected to connect, got error instead: %s", err.Error())
	}
	adapter.Close(ctx)

	adapter = NewWikiStreamAdapterWithClient(logger, db, &MockErrorClient{}, "https://example.com")
	if err := adapter.Connect(ctx); err == nil || !errors.Is(err, ErrMockConnectErr) {
		t.Errorf("Expected error '%s', got '%s'", ErrMockConnectErr.Error(), err.Error())
	}
	adapter.Close(ctx) // This close isn't needed, but it's consistant with the adapter pattern
}

func TestWikiStreamAdapterConume(t *testing.T) {
	r, w := io.Pipe()

	client := &MockBufferClient{buffer: r}
	logger := slog.New(slog.NewJSONHandler(t.Output(), nil))
	db := database.NewWikiStatsDB()
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		payload := &models.WikiStreamMessage{}
		payload.Meta = models.WikiStreamMessageMeta{
			ID: "MockID1",
		}
		payload.User = "Steve"
		payload.ServerName = "MockServer1"
		payload.Bot = false

		b, err := json.Marshal(payload)
		if err != nil {
			t.Error(err)
			w.CloseWithError(err)
			return
		}

		fmt.Fprintf(w, "%s%s\n", dataTag, b)
		w.Close()
	}()
	time.Sleep(time.Second / 4) // Wait for work

	adapter := NewWikiStreamAdapterWithClient(logger, db, client, "https://example.com")
	if err := adapter.Connect(ctx); err != nil {
		t.Errorf("Expected to connect, got error instead: %s", err.Error())
	}

	errCh := make(chan error, 1)
	go func() {
		if err := adapter.Consume(ctx); err != nil {
			errCh <- err
		}
	}()
	time.Sleep(time.Second / 4) // Wait for work

	cancel() // Cancel the consume

	err := <-errCh
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		t.Errorf("Expected to consume, got error instead: %s", err.Error())
	}

	msg, user, bots, servers := db.GetCounts()

	if msg != 1 {
		t.Errorf("Expected 1 message, got %s", strconv.Itoa(msg))
	}
	if user != 1 {
		t.Errorf("Expected 1 user, got %s", strconv.Itoa(user))
	}
	if bots != 0 {
		t.Errorf("Expected 0 bots, got %s", strconv.Itoa(bots))
	}
	if servers != 1 {
		t.Errorf("Expected 1 server, got %s", strconv.Itoa(servers))
	}
}
