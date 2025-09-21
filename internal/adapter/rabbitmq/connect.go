// internal/adapter/rabbitmq/connect.go
package rabbitmq

import (
	"context"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"time"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

// Connection wraps AMQP connection with recovery functionality
type Connection struct {
	log       logger.Logger
	conn      *amqp091.Connection
	channel   *amqp091.Channel
	cfg       config.Config
	closed    bool
	reconnect chan struct{}
	metrics   *RabbitMQMetrics
}

// NewConnection creates a new RabbitMQ connection
func NewConnection(ctx context.Context, cfg config.Config) (*Connection, error) {
	log := logger.InitLogger("rabbitmq", logger.LevelDebug)

	connection := &Connection{
		log:       log,
		cfg:       cfg,
		closed:    false,
		reconnect: make(chan struct{}, 1),
		metrics:   NewRabbitMQMetrics(),
	}

	if err := connection.connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	go connection.handleReconnect(ctx)

	return connection, nil
}

// connect establishes a connection to RabbitMQ and creates a channel
func (c *Connection) connect(ctx context.Context) error {
	c.log.Info(ctx, types.ActionRabbitMQConnecting, "connecting to RabbitMQ")

	rabbitCfg := c.cfg.RabbitMQ
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/",
		rabbitCfg.User,
		rabbitCfg.Password,
		rabbitCfg.Host,
		rabbitCfg.Port,
	)

	var err error
	c.conn, err = amqp091.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Monitor connection closure for auto-recovery
	go func() {
		// Get notification about connection closure
		connClosed := c.conn.NotifyClose(make(chan *amqp091.Error, 1))

		select {
		case <-ctx.Done():
			c.log.Info(ctx, types.ActionRabbitMQDisconnected, "context cancelled, stopping connection monitoring")
			return
		case err := <-connClosed:
			if c.closed {
				c.log.Info(ctx, types.ActionRabbitMQDisconnected, "connection closed gracefully")
				return
			}
			c.log.Error(ctx, types.ActionRabbitMQDisconnected, "connection closed unexpectedly", err)
			// Trigger reconnect
			c.reconnect <- struct{}{}
		}
	}()

	c.metrics.RecordSuccessfulConnect()
	c.log.Info(ctx, types.ActionRabbitMQConnected, "successfully connected to RabbitMQ")
	return nil
}

// handleReconnect handles reconnection to RabbitMQ
func (c *Connection) handleReconnect(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.log.Info(ctx, types.ActionGracefulShutdown, "context cancelled, stopping reconnect handler")
			return
		case <-c.reconnect:
			if c.closed {
				return
			}

			backoff := 1 * time.Second
			maxBackoff := 30 * time.Second

			for {
				c.metrics.RecordReconnectAttempt()
				c.log.Info(ctx, types.ActionRabbitMQReconnecting,
					"attempting to reconnect to RabbitMQ",
					"backoff", backoff.String(),
				)

				// Close old connections if they still exist
				if c.channel != nil {
					c.channel.Close()
				}
				if c.conn != nil {
					c.conn.Close()
				}

				// Attempt reconnection
				err := c.connect(ctx)
				if err == nil {
					c.log.Info(ctx, types.ActionRabbitMQReconnected, "successfully reconnected to RabbitMQ")
					break
				}

				c.log.Error(ctx, types.ActionRabbitMQReconnectFailed, "failed to reconnect", err,
					"next_attempt", backoff.String(),
				)

				select {
				case <-ctx.Done():
					c.log.Info(ctx, types.ActionGracefulShutdown, "context cancelled during reconnect")
					return
				case <-time.After(backoff):
					// Increase backoff time for next attempt with a limit
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
			}
		}
	}
}

// Channel returns the current RabbitMQ channel
func (c *Connection) Channel() *amqp091.Channel {
	return c.channel
}

// PublishWithContext publishes a message with context and tracks metrics
func (c *Connection) PublishWithContext(ctx context.Context, exchange, routingKey string, mandatory, immediate bool, msg amqp091.Publishing) error {
	err := c.channel.PublishWithContext(ctx, exchange, routingKey, mandatory, immediate, msg)
	if err != nil {
		c.metrics.IncrementFailed()
		return err
	}

	c.metrics.IncrementPublished()
	return nil
}

// GetMetrics returns connection metrics
func (c *Connection) GetMetrics() map[string]interface{} {
	return c.metrics.GetMetrics()
}

// Close closes the RabbitMQ connection
func (c *Connection) Close() error {
	c.closed = true

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			return fmt.Errorf("error closing channel: %w", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("error closing connection: %w", err)
		}
	}

	return nil
}
