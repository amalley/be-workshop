package scylla

import (
	"time"

	"github.com/gocql/gocql"
)

type ScyllaOption func(*ScyllaOptions)

type ScyllaOptions struct {
	Host                     string
	Keyspace                 string
	RetryTime                time.Duration
	ConnectionTimeout        time.Duration
	ClusterConsistency       gocql.Consistency
	HostSelectPolicy         gocql.HostSelectionPolicy
	DisableInitialHostLookup bool
}

func WithHost(host string) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.Host = host
	}
}

func WithKeyspace(keyspace string) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.Keyspace = keyspace
	}
}

func WithRetryTime(retryTime time.Duration) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.RetryTime = retryTime
	}
}

func WithConnectionTimeout(timeout time.Duration) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.ConnectionTimeout = timeout
	}
}

func WithClusterConsistency(consistency gocql.Consistency) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.ClusterConsistency = consistency
	}
}

func WithHostSelectionPolicy(policy gocql.HostSelectionPolicy) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.HostSelectPolicy = policy
	}
}

func WithDisableInitialHostLookup(disable bool) ScyllaOption {
	return func(opts *ScyllaOptions) {
		opts.DisableInitialHostLookup = disable
	}
}

func defaultOptions() *ScyllaOptions {
	return &ScyllaOptions{
		Host:                     "localhost:9042",
		Keyspace:                 "wikistats",
		RetryTime:                5 * time.Second,
		ConnectionTimeout:        5 * time.Second,
		ClusterConsistency:       gocql.Quorum,
		HostSelectPolicy:         gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy()),
		DisableInitialHostLookup: false,
	}
}

func applyOptions(cfg *ScyllaOptions, opts ...ScyllaOption) *ScyllaOptions {
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
