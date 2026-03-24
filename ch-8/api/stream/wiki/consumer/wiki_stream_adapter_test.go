package consumer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/amalley/be-workshop/ch-8/api/stream/wiki"
	"github.com/amalley/be-workshop/ch-8/api/stream/wiki/consumer"
	"github.com/amalley/be-workshop/ch-8/models"
	"github.com/amalley/be-workshop/ch-8/models/gen/pbwiki"
	"github.com/gocql/gocql"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	rawSSE = `{"$schema":"/test/schema/1.0.0","meta":{"uri":"https://test.wiki/wiki/Test_Page","request_id":"deadbeef-0000-0000-0000-000000000001","id":"deadbeef-0000-0000-0000-000000000001","domain":"test.wiki","stream":"test.stream","dt":"2026-01-01T12:00:00.000Z","topic":"test.topic","partition":0,"offset":123456789},"id":999999999,"type":"edit","namespace":0,"title":"Test Page","title_url":"https://test.wiki/wiki/Test_Page","comment":"This is a test comment for unit testing purposes","timestamp":1735732800,"user":"TestUserBot","bot":true,"notify_url":"https://test.wiki/w/index.php?diff=999&oldid=888&rcid=999999999","server_url":"https://test.wiki","server_name":"test.wiki","server_script_path":"/w","wiki":"testwiki","parsedcomment":"<a href=\"/wiki/Test_Page\" title=\"Test Page\">Test Page</a> modified by test bot"}`
)

var (
	protojsonUnmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

type MockDBWriter struct {
	mu     sync.Mutex
	totals models.WikiStatsCounts
}

func (m *MockDBWriter) InsertStats(ctx context.Context, stats *models.WikiStatsCounts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totals.Messages += stats.Messages
	m.totals.Users += stats.Users
	m.totals.Bots += stats.Bots
	m.totals.Servers += stats.Servers
	return nil
}

func (m *MockDBWriter) CreateUser(ctx context.Context, username, password string) error {
	return nil
}

func (m *MockDBWriter) DeleteUser(ctx context.Context, userID gocql.UUID) error {
	return nil
}

func TestWikiStreamAdapterConsumer(t *testing.T) {
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

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers),
		kgo.DefaultProduceTopic("test-topic"),
		kgo.RecordRetries(5),
		kgo.MetadataMaxAge(5*time.Second),
	)

	if err != nil {
		t.Fatalf("failed to create producer client: %v", err)
	}
	defer client.Close()

	adminClient := kadm.NewClient(client)
	resp, err := adminClient.CreateTopics(ctx, 1, 1, nil, "test-topic")
	if err != nil || resp.Error() != nil {
		if err == nil {
			err = resp.Error()
		}
		t.Fatalf("failed to create topic: %v", err)
	}

	const produceCount = 10

	var message pbwiki.WikiStreamMessage
	if err := protojsonUnmarshalOptions.Unmarshal([]byte(rawSSE), &message); err != nil {
		t.Fatalf("error unmarshaling raw SSE: %v", err)
	}

	protoData, err := proto.Marshal(&message)
	if err != nil {
		t.Fatalf("error marshaling message to protobuf: %v", err)
	}

	records := make([]*kgo.Record, produceCount)
	for i := range records {
		records[i] = &kgo.Record{
			Value: protoData,
		}
	}

	// Produce a test message
	results := client.ProduceSync(ctx, records...)
	if len(results) > 0 && results[0].Err != nil {
		t.Fatalf("failed to produce message: %v", results[0].Err)
	}

	// This will be used to ensure the adapter is consuming messages and writing to the database
	db := &MockDBWriter{}

	adapter := consumer.NewStreamAdapter(
		wiki.WithBrokers([]string{brokers}),
		wiki.WithTopic("test-topic"),
		wiki.WithConsumerGroupID("test-group"),
		wiki.WithDBWriter(db),
		wiki.WithMaxPollRecords(produceCount),
	)

	if err := adapter.Connect(ctx); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer adapter.Close(ctx)

	consumeCtx, consumeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer consumeCancel()

	// Start consuming in a separate goroutine
	go func() {
		if err := adapter.Consume(consumeCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("consume error: %v", err)
		}
	}()

	success := false
	for range 10 {
		db.mu.Lock()
		if db.totals.Messages == produceCount {
			success = true
			db.mu.Unlock()
			break
		}
		db.mu.Unlock()
		time.Sleep(1 * time.Second)
	}

	if !success {
		t.Errorf("Timed out waiting for messages. Got %d, want %d", db.totals.Messages, produceCount)
	}

	if db.totals.Messages != produceCount {
		t.Errorf("expected %d messages, got %d", produceCount, db.totals.Messages)
	}
	if db.totals.Users != produceCount {
		t.Errorf("expected %d users, got %d", produceCount, db.totals.Users)
	}
	if db.totals.Bots != produceCount {
		t.Errorf("expected %d bots, got %d", produceCount, db.totals.Bots)
	}
	if db.totals.Servers != produceCount {
		t.Errorf("expected %d servers, got %d", produceCount, db.totals.Servers)
	}
}
