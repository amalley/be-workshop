package wiki

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/amalley/be-workshop/ch-5/api/database"
	"github.com/amalley/be-workshop/ch-5/api/web"
)

type WikiStreamOption func(*WikiStreamOptions)

type WikiStreamOptions struct {
	DBWriter          database.Writer
	Logger            *slog.Logger
	URL               *url.URL
	Doer              web.RequestDoer
	Topic             string
	ConsumerGroupID   string
	Brokers           []string
	RetryAttempts     int
	MaxPollRecords    int
	FetchMaxWait      time.Duration
	FetchMinBytes     int32
	AutoTopicCreation bool
}

func WithAutoTopicCreation(enabled bool) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.AutoTopicCreation = enabled
	}
}

func WithDBWriter(writer database.Writer) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.DBWriter = writer
	}
}

func WithFetchMaxWait(wait time.Duration) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.FetchMaxWait = wait
	}
}

func WithFetchMinBytes(bytes int32) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.FetchMinBytes = bytes
	}
}

func WithDoer(doer web.RequestDoer) WikiStreamOption {
	if doer == nil {
		doer = http.DefaultClient
	}
	return func(opts *WikiStreamOptions) {
		opts.Doer = doer
	}
}

func WithRetryAttempts(attempts int) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.RetryAttempts = attempts
	}
}

func WithLogger(logger *slog.Logger) WikiStreamOption {
	if logger == nil {
		logger = slog.Default()
	}
	return func(opts *WikiStreamOptions) {
		opts.Logger = logger
	}
}

func WithURL(url *url.URL) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.URL = url
	}
}

func WithTopic(topic string) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.Topic = topic
	}
}

func WithBrokers(brokers []string) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.Brokers = brokers
	}
}

func WithMaxPollRecords(max int) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.MaxPollRecords = max
	}
}

func WithConsumerGroupID(groupID string) WikiStreamOption {
	return func(opts *WikiStreamOptions) {
		opts.ConsumerGroupID = groupID
	}
}

func DefaultWikiStreamOptions() *WikiStreamOptions {
	return &WikiStreamOptions{
		DBWriter:          nil,
		Logger:            slog.Default(),
		URL:               nil,
		Topic:             "",
		ConsumerGroupID:   "wikistats-group",
		Brokers:           []string{},
		RetryAttempts:     5,
		Doer:              http.DefaultClient,
		FetchMaxWait:      100 * time.Millisecond,
		FetchMinBytes:     1024 * 10, // 10KB
		MaxPollRecords:    100,       // 100 messages
		AutoTopicCreation: true,
	}
}

func ApplyWikiStreamOptions(opts *WikiStreamOptions, options ...WikiStreamOption) *WikiStreamOptions {
	for _, opt := range options {
		opt(opts)
	}
	return opts
}
