package producer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/stream/wiki"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type MockStream struct {
	response string
}

func (m *MockStream) Do(req *http.Request) (*http.Response, error) {
	r, w := io.Pipe()
	go func() {
		resp := []byte(m.response)
		if !bytes.HasSuffix(resp, []byte("\n\n")) {
			resp = append(resp, []byte("\n\n")...)
		}
		for {
			select {
			case <-req.Context().Done():
				w.Close()
				return
			default:
				// Write an event to the pipe
				_, err := w.Write(resp)
				if err != nil {
					return
				}
				// Simulate a delay between events
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       r,
	}, nil
}

func TestWikiProducerIntegration(t *testing.T) {
	const rawSSE = `data: {"meta": {"id": "1"}, "user": "testuser", "server_name": "testserver", "bot": false}`
	const expectedJSON = `{"meta": {"id": "1"}, "user": "testuser", "server_name": "testserver", "bot": false}`

	ctx := context.Background()

	container, err := redpanda.Run(ctx, "redpandadata/redpanda:v23.3.13")
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %v", err)
		}
	}()

	brokers, err := container.KafkaSeedBroker(ctx)
	if err != nil {
		t.Fatalf("failed to get broker address: %v", err)
	}

	u, _ := url.Parse("https://example.com")
	adapter := NewWikiStreamAdapterWithClient(
		wiki.WithURL(u),
		wiki.WithDoer(&MockStream{response: rawSSE}),
		wiki.WithTopic("test-topic"),
		wiki.WithBrokers([]string{brokers}),
		wiki.WithRetryAttempts(20),
		wiki.WithAutoTopicCreation(false),
	)

	if err := adapter.Connect(ctx); err != nil {
		t.Fatalf("failed to connect stream adapter: %v", err)
	}

	adminClinet := kadm.NewClient(adapter.client)

	resp, err := adminClinet.CreateTopics(ctx, 1, 1, nil, "test-topic")
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}

	if err := resp.Error(); err != nil {
		t.Fatalf("error creating topic: %v", err)
	}

	var grp sync.WaitGroup
	grp.Go(func() {
		wait := time.NewTimer(2 * time.Second)
		defer wait.Stop()

		consumeCtx, consumeCancel := context.WithCancel(ctx)
		defer consumeCancel()

		go func() {
			select {
			case <-consumeCtx.Done():
				return
			case <-wait.C:
				consumeCancel()
			}
		}()

		if err := adapter.Consume(consumeCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("failed to consume from stream adapter: %v", err)
		}
	})
	grp.Wait()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers),
		kgo.ConsumeTopics("test-topic"),
		kgo.FetchMaxWait(5*time.Second),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("failed to create Kafka client: %v", err)
	}
	defer client.Close()

	pollCtx, pollCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pollCancel()

	fetches := client.PollRecords(pollCtx, 1)
	if errs := fetches.Errors(); len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("fetch error: %v", err.Err)
		}
	}

	var found bool
	fetches.EachRecord(func(record *kgo.Record) {
		if string(bytes.TrimSpace(record.Value)) == expectedJSON {
			found = true
		}
	})

	if !found {
		t.Fatal("expected to find the produced message, but it was not found")
	}
}
