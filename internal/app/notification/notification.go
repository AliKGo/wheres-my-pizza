// internal/app/notification/notification.go
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wheres-my-pizza/internal/adapter/rabbitmq/notification_subscriber"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

// NotificationApp represents the notification subscriber application
type NotificationApp struct {
	ctx        context.Context
	cancel     context.CancelFunc
	logger     logger.Logger
	subscriber *notification_subscriber.NotificationSubscriber
}

// NewNotificationApp creates a new notification application
func NewNotificationApp() *NotificationApp {
	cfg, err := config.ParseYAML()
	if err != nil {
		config.PrintYAMLHelp()
		slog.Error("failed to configure application", "error", err)
		os.Exit(1)
	}

	logger := logger.InitLogger("Notification Subscriber", logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())

	// Connect to RabbitMQ
	subscriber, err := notification_subscriber.NewNotificationSubscriber(ctx, cfg)
	if err != nil {
		cancel()
		logger.Error(ctx, types.ActionRabbitMQConnectFailed, "failed to connect to RabbitMQ", err)
		os.Exit(1)
	}

	return &NotificationApp{
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		subscriber: subscriber,
	}
}

// Start begins the notification subscriber operation
func (app *NotificationApp) Start() {
	// Start notification consumption
	go app.consumeNotifications()

	// Wait for signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	app.logger.Info(app.ctx, types.ActionGracefulShutdown, "service is shutting down")
	app.cancel()

	// Close connections
	if app.subscriber != nil {
		if err := app.subscriber.Close(); err != nil {
			app.logger.Error(app.ctx, types.ActionGracefulShutdown, "error closing RabbitMQ connection", err)
		}
	}
}

// consumeNotifications starts consuming notifications from the queue
func (app *NotificationApp) consumeNotifications() {
	err := app.subscriber.ConsumeNotifications(app.ctx, func(delivery amqp091.Delivery) error {
		var statusUpdate models.StatusUpdate

		if err := json.Unmarshal(delivery.Body, &statusUpdate); err != nil {
			app.logger.Error(app.ctx, types.ActionMessageProcessingFailed, "failed to unmarshal status update", err)
			return err
		}

		app.logger.Debug(app.ctx, types.ActionNotificationReceived, "received status update notification",
			"order_number", statusUpdate.OrderNumber,
			"old_status", statusUpdate.OldStatus,
			"new_status", statusUpdate.NewStatus,
			"changed_by", statusUpdate.ChangedBy,
		)

		// Format human-readable message
		message := fmt.Sprintf("Notification for order %s: Status changed from '%s' to '%s' by %s.",
			statusUpdate.OrderNumber,
			statusUpdate.OldStatus,
			statusUpdate.NewStatus,
			statusUpdate.ChangedBy,
		)

		// Add estimated completion time if available
		if !statusUpdate.EstimatedCompletion.IsZero() {
			message += fmt.Sprintf(" Estimated completion time: %s",
				statusUpdate.EstimatedCompletion.Format(time.Kitchen),
			)
		}

		// Output notification to console
		fmt.Println(message)

		return nil
	})

	if err != nil && err != context.Canceled {
		app.logger.Error(app.ctx, types.ActionRabbitMQConsumeFailed, "error consuming notifications", err)
		app.cancel() // Initiate application shutdown
	}
}
