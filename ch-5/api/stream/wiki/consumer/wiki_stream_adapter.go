package consumer

import (
	"context"
	"errors"

	"github.com/amalley/be-workshop/ch-5/api/stream"
	"github.com/amalley/be-workshop/ch-5/api/stream/wiki"
	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	ErrAlreadyConnected = errors.New("stream adapter is already connected")
	ErrNotConnected     = errors.New("stream adapter is not connected")
)

var _ stream.StreamAdapter = &WikiStreamAdapterConsumer{}

type WikiStreamAdapterConsumer struct {
	cfg    *wiki.WikiStreamOptions
	client *kgo.Client
}

func NewStreamAdapter(options ...wiki.WikiStreamOption) *WikiStreamAdapterConsumer {
	cfg := wiki.ApplyWikiStreamOptions(wiki.DefaultWikiStreamOptions(), options...)
	cfg.Logger = cfg.Logger.With("src", "WikiStreamAdapterConsumer")
	return &WikiStreamAdapterConsumer{cfg: cfg, client: nil}
}

func (a *WikiStreamAdapterConsumer) Connect(ctx context.Context) error {
	if a.client != nil {
		return ErrAlreadyConnected
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(a.cfg.Brokers...),
		kgo.ConsumeTopics(a.cfg.Topic),
		kgo.FetchMaxWait(a.cfg.FetchMaxWait),
		kgo.FetchMinBytes(a.cfg.FetchMinBytes),
	)
	if err != nil {
		return err
	}
	a.client = client

	return nil
}

func (a *WikiStreamAdapterConsumer) Close(ctx context.Context) error {
	if a.client == nil {
		return nil
	}

	a.client.Close()
	a.client = nil

	return nil
}

func (a *WikiStreamAdapterConsumer) Consume(ctx context.Context) error {
	if a.client == nil {
		return ErrNotConnected
	}

	return nil
}
