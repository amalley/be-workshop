package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/AMalley/be-workshop/ch-5/models"
	"github.com/gocql/gocql"
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

type MockDBAdapter struct {
	messages map[string]struct{}
	users    map[string]struct{}
	servers  map[string]struct{}
	bots     map[string]struct{}
}

func NewMockDBAdapter() *MockDBAdapter {
	return &MockDBAdapter{
		messages: make(map[string]struct{}),
		users:    make(map[string]struct{}),
		servers:  make(map[string]struct{}),
		bots:     make(map[string]struct{}),
	}
}

func (a *MockDBAdapter) Connect(ctx context.Context) error {
	return nil
}

func (a *MockDBAdapter) Close(ctx context.Context) error {
	return nil
}

func (a *MockDBAdapter) IsReady() bool {
	return true
}

func (a *MockDBAdapter) InsertStats(ctx context.Context, stats models.WikiStatsModel) error {
	a.messages[stats.Message] = struct{}{}
	a.servers[stats.Server] = struct{}{}

	if stats.IsBot {
		a.bots[stats.User] = struct{}{}
	} else {
		a.users[stats.User] = struct{}{}
	}

	return nil
}

func (a *MockDBAdapter) GetStats(ctx context.Context) (models.WikiStatsCounts, error) {
	return models.WikiStatsCounts{
		Messages: len(a.messages),
		Users:    len(a.users),
		Servers:  len(a.servers),
		Bots:     len(a.bots),
	}, nil
}

func (a *MockDBAdapter) CreateUser(ctx context.Context, n, p string) error {
	return nil
}

func (a *MockDBAdapter) DeleteUser(ctx context.Context, n gocql.UUID) error {
	return nil
}

func (a *MockDBAdapter) GetUser(ctx context.Context, n string) (models.User, bool, error) {
	return models.User{}, false, nil
}

func (a *MockDBAdapter) GetUserByID(ctx context.Context, userID gocql.UUID) (models.User, bool, error) {
	return models.User{}, false, nil
}

func TestWikiStreamAdapterConnect(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(t.Output(), nil))
	ctx := context.Background()
	db := NewMockDBAdapter()

	u, _ := url.Parse("https://example.com")

	adapter := NewWikiStreamAdapterWithClient(logger, db, &MockClient{}, u)
	if err := adapter.Connect(ctx); err != nil {
		t.Errorf("Expected to connect, got error instead: %s", err.Error())
	}
	adapter.Close(ctx)

	adapter = NewWikiStreamAdapterWithClient(logger, db, &MockErrorClient{}, u)
	if err := adapter.Connect(ctx); err == nil || !errors.Is(err, ErrMockConnectErr) {
		t.Errorf("Expected error '%s', got '%s'", ErrMockConnectErr.Error(), err.Error())
	}
	adapter.Close(ctx) // This close isn't needed, but it's consistant with the adapter pattern
}

func TestWikiStreamAdapterConume(t *testing.T) {
	r, w := io.Pipe()

	client := &MockBufferClient{buffer: r}
	logger := slog.New(slog.NewJSONHandler(t.Output(), nil))
	ctx, cancel := context.WithCancel(context.Background())
	db := NewMockDBAdapter()

	go func() {
		b, err := json.Marshal(&models.WikiStreamMessage{
			Meta: models.WikiStreamMessageMeta{
				ID: "MockID1",
			},
			User:       "Steve",
			ServerName: "MockServer1",
			Bot:        false,
		})
		if err != nil {
			t.Error(err)
			w.CloseWithError(err)
			return
		}

		fmt.Fprintf(w, "%s%s\n", dataTag, b)
		w.Close()
	}()
	time.Sleep(time.Second / 4) // Wait for work

	u, _ := url.Parse("https://example.com")

	adapter := NewWikiStreamAdapterWithClient(logger, db, client, u)
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

	stats, err := db.GetStats(ctx)

	if err != nil {
		t.Errorf("Expected to get stats, got error instead: %s", err.Error())
	}

	if stats.Messages != 1 {
		t.Errorf("Expected 1 message, got %s", strconv.Itoa(stats.Messages))
	}
	if stats.Users != 1 {
		t.Errorf("Expected 1 user, got %s", strconv.Itoa(stats.Users))
	}
	if stats.Bots != 0 {
		t.Errorf("Expected 0 bots, got %s", strconv.Itoa(stats.Bots))
	}
	if stats.Servers != 1 {
		t.Errorf("Expected 1 server, got %s", strconv.Itoa(stats.Servers))
	}
}
