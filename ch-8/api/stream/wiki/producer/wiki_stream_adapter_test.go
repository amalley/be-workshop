package producer_test

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

	"github.com/amalley/be-workshop/ch-8/api/stream/wiki"
	"github.com/amalley/be-workshop/ch-8/api/stream/wiki/producer"
	"github.com/amalley/be-workshop/ch-8/models/gen/pbwiki"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

const (
	testId = "deadbeef-0000-0000-0000-000000000001"
	rawSSE = `data: {"$schema":"/test/schema/1.0.0","meta":{"uri":"https://test.wiki/wiki/Test_Page","request_id":"deadbeef-0000-0000-0000-000000000001","id":"deadbeef-0000-0000-0000-000000000001","domain":"test.wiki","stream":"test.stream","dt":"2026-01-01T12:00:00.000Z","topic":"test.topic","partition":0,"offset":123456789},"id":999999999,"type":"edit","namespace":0,"title":"Test Page","title_url":"https://test.wiki/wiki/Test_Page","comment":"This is a test comment for unit testing purposes","timestamp":1735732800,"user":"TestUserBot","bot":true,"notify_url":"https://test.wiki/w/index.php?diff=999&oldid=888&rcid=999999999","server_url":"https://test.wiki","server_name":"test.wiki","server_script_path":"/w","wiki":"testwiki","parsedcomment":"<a href=\"/wiki/Test_Page\" title=\"Test Page\">Test Page</a> modified by test bot"}`
)

type MockStream struct {
	response []byte
}

func (m *MockStream) Do(req *http.Request) (*http.Response, error) {
	r, w := io.Pipe()
	go func() {
		resp := m.response
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
	adapter := producer.NewWikiStreamAdapterWithClient(
		wiki.WithURL(u),
		wiki.WithDoer(&MockStream{response: []byte(rawSSE)}),
		wiki.WithTopic("test-topic"),
		wiki.WithBrokers([]string{brokers}),
		wiki.WithRetryAttempts(20),
		wiki.WithAutoTopicCreation(false),
	)

	if err := adapter.Connect(ctx); err != nil {
		t.Fatalf("failed to connect stream adapter: %v", err)
	}

	adminClient := kadm.NewClient(adapter.Client())

	resp, err := adminClient.CreateTopics(ctx, 1, 1, nil, "test-topic")
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
		var message pbwiki.WikiStreamMessage
		if err := proto.Unmarshal(record.Value, &message); err != nil {
			t.Errorf("error unmarshaling record: %v", err)
			return
		}

		id := "not found"

		if message.Meta != nil && message.Meta.Id != nil {
			id = *message.Meta.Id
		}

		if id == testId {
			found = true
		}
	})

	if !found {
		t.Fatal("expected to find the produced message, but it was not found")
	}
}
