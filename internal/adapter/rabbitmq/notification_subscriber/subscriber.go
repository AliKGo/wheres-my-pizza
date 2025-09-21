// internal/adapter/rabbitmq/notification_subscriber/subscriber.go
package notification_subscriber

import (
	"context"
	"errors"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"wheres-my-pizza/internal/adapter/rabbitmq"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

// NotificationSubscriber handles subscribing to notifications
type NotificationSubscriber struct {
	conn *rabbitmq.Connection
	log  logger.Logger
}

// NewNotificationSubscriber creates a new notification subscriber
func NewNotificationSubscriber(ctx context.Context, cfg config.Config) (*NotificationSubscriber, error) {
	log := logger.InitLogger("notification_subscriber", logger.LevelDebug)

	conn, err := rabbitmq.NewConnection(ctx, cfg)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQConnectFailed, "failed to create RabbitMQ connection", err)
		return nil, fmt.Errorf("failed to create RabbitMQ connection: %w", err)
	}

	// Set up RabbitMQ
	if err := rabbitmq.SetupRabbitMQ(ctx, conn, log); err != nil {
		conn.Close()
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to setup RabbitMQ", err)
		return nil, fmt.Errorf("failed to setup RabbitMQ: %w", err)
	}

	return &NotificationSubscriber{
		conn: conn,
		log:  log,
	}, nil
}

// ConsumeNotifications starts consuming notifications from the queue
func (s *NotificationSubscriber) ConsumeNotifications(ctx context.Context, handler func(delivery amqp091.Delivery) error) error {
	ch := s.conn.Channel()

	// Start consuming from notifications queue
	msgs, err := ch.Consume(
		rabbitmq.NotificationsQueue, // queue name
		"",                          // consumer name (empty for auto-generation)
		false,                       // auto-ack
		false,                       // exclusive
		false,                       // no-local
		false,                       // no-wait
		nil,                         // arguments
	)
	if err != nil {
		s.log.Error(ctx, types.ActionRabbitMQConsumeFailed, "failed to start consuming notifications", err)
		return fmt.Errorf("failed to start consuming notifications: %w", err)
	}

	s.log.Info(ctx, types.ActionRabbitMQConsumeStarted, "started consuming from notifications queue")

	for {
		select {
		case <-ctx.Done():
			s.log.Info(ctx, types.ActionGracefulShutdown, "stopping consumption due to context cancellation")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				s.log.Error(ctx, types.ActionRabbitMQConsumeFailed, "notification channel closed", errors.New("channel closed"))
				return errors.New("notification channel closed")
			}

			// Process notification using provided handler
			s.log.Debug(ctx, types.ActionNotificationReceived, "received notification")

			if err := handler(msg); err != nil {
				s.log.Error(ctx, types.ActionMessageProcessingFailed, "failed to process notification", err)
				// Do not return to queue, as this is not critical for business logic
				if err := msg.Nack(false, false); err != nil {
					s.log.Error(ctx, types.ActionRabbitMQNackFailed, "failed to nack notification", err)
				}
				continue
			}

			// Successful processing, acknowledge message
			if err := msg.Ack(false); err != nil {
				s.log.Error(ctx, types.ActionRabbitMQAckFailed, "failed to ack notification", err)
			}
		}
	}
}

// Close closes the connection to RabbitMQ
func (s *NotificationSubscriber) Close() error {
	return s.conn.Close()
}
