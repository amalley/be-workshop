package scylla_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/amalley/be-workshop/ch-3/api/database/scylla"
	"github.com/gocql/gocql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const scyllaContainerDuration = 3 * time.Minute

func newContainer(ctx context.Context) (testcontainers.Container, error) {
	scyllaContainer, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "scylladb/scylla:latest",
				Cmd: []string{
					"--developer-mode", "1",
					"--smp", "1",
					"--memory", "1G",
					"--overprovisioned", "1",
				},
				ExposedPorts: []string{
					"9042/tcp",
				},
				WaitingFor: wait.ForAll(
					wait.ForLog("Starting Messaging Service"),
					wait.ForListeningPort("9042/tcp"),
				).WithDeadline(scyllaContainerDuration),
			},
			Started: true,
		},
	)
	return scyllaContainer, err
}

func TestScyllaIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scyllaContainerDuration)
	defer cancel()

	container, err := newContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9042")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	address := net.JoinHostPort(host, port.Port())

	logger := slog.Default()
	logger.Info("Scylla container started", "address", address)

	adapter := scylla.NewScyllaDatabaseAdapter(logger,
		scylla.WithHost(address),
		scylla.WithKeyspace("wikistats"),
		scylla.WithClusterConsistency(gocql.One),
		scylla.WithConnectionTimeout(5*time.Second),
		scylla.WithRetryTime(5*time.Second),
		scylla.WithDisableInitialHostLookup(true),
		scylla.WithHostSelectionPolicy(gocql.RoundRobinHostPolicy()),
	)

	if err := adapter.Connect(ctx); err != nil {
		t.Fatalf("failed to connect to Scylla: %v", err)
	}
	defer adapter.Close(ctx)

	t.Run("User Lifecycle and View Sync", func(t *testing.T) {
		username := "testuser"
		password := "testpassword"

		if err := adapter.CreateUser(ctx, username, password); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		userByName, exists, err := adapter.GetUser(ctx, username)
		if err != nil {
			t.Fatalf("failed to get user by ID: %v", err)
		}
		if !exists {
			t.Fatalf("user does not exist")
		}

		userByID, exists, err := adapter.GetUserByID(ctx, userByName.ID)
		if err != nil {
			t.Fatalf("failed to get user by ID: %v", err)
		}
		if !exists {
			t.Fatalf("user does not exist")
		}

		if userByID.ID != userByName.ID {
			t.Fatalf("user IDs do not match")
		}

		if userByID.Username != username {
			t.Fatalf("usernames do not match")
		}

		if err := adapter.DeleteUser(ctx, userByName.ID); err != nil {
			t.Fatalf("failed to delete user: %v", err)
		}

		_, exists, err = adapter.GetUser(ctx, username)
		if err != nil {
			t.Fatalf("failed to get user by ID: %v", err)
		}
		if exists {
			t.Fatalf("user should not exist")
		}
	})
}
