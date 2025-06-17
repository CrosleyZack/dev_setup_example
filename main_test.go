package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ExampleTestSuite struct {
	suite.Suite

	ctx       context.Context
	cancel    context.CancelFunc
	container testcontainers.Container
}

func (s *ExampleTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	conf := &PgContainerConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "test",
		User:     "test",
		Password: "test",
	}
	c, err := getContainer(s.ctx, conf)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), c)
	s.container = *c
	err = validateContainer(s.ctx, conf)
	assert.NoError(s.T(), err)
}

func (s *ExampleTestSuite) TearDownSuite() {
	s.container.Terminate(s.ctx)
	s.cancel()
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(ExampleTestSuite))
}

func (s *ExampleTestSuite) TestExample() {
	assert.True(s.T(), true)
}

type PgContainerConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

func (c *PgContainerConfig) GetConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Name,
	)
}

// getContainer returns a running container
func getContainer(ctx context.Context, pgConfig *PgContainerConfig) (*testcontainers.Container, error) {
	strPort := strconv.Itoa(pgConfig.Port)
	req := testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:17-alpine",
			ExposedPorts: []string{strPort},
			WaitingFor:   wait.ForListeningPort(nat.Port(strPort)),
			Env: map[string]string{
				"POSTGRES_USER":     pgConfig.User,
				"POSTGRES_PASSWORD": pgConfig.Password,
				"POSTGRES_DB":       pgConfig.Name,
			},
			// Does not resolve the `chmod: ... Operation not permitted` error
			// Privileged: true,
			// User: "1000:1000",
		},
	}
	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		return nil, err
	}
	return &container, nil
}

// verify we can connect to the container
func validateContainer(ctx context.Context, config *PgContainerConfig) error {
	var err error
	var conn *pgx.Conn
	for {
		conn, err = pgx.Connect(ctx, config.GetConnectionString())
		if err == nil {
			break
		}
		var pgErr *pgconn.PgError
		// If the database is still starting up - wait a bit and try again
		if errors.As(err, &pgErr) && pgErr.Code == "57P03" {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
	defer conn.Close(ctx) //nolint:all
	return err
}
