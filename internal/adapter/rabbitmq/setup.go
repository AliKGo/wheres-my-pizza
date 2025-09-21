// internal/adapter/rabbitmq/setup.go
package rabbitmq

import (
	"context"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/logger"
)

// Exchange and queue constants
const (
	OrdersTopicExchange         = "orders_topic"
	NotificationsFanoutExchange = "notifications_fanout"

	KitchenQueue         = "kitchen_queue"
	KitchenDineInQueue   = "kitchen_dine_in_queue"
	KitchenTakeoutQueue  = "kitchen_takeout_queue"
	KitchenDeliveryQueue = "kitchen_delivery_queue"

	NotificationsQueue = "notifications_queue"
	DeadLetterQueue    = "dead_letter_queue"
	DeadLetterExchange = "dead_letter_exchange"
)

// SetupRabbitMQ configures all necessary exchanges, queues and bindings
func SetupRabbitMQ(ctx context.Context, conn *Connection, log logger.Logger) error {
	ch := conn.Channel()

	log.Info(ctx, types.ActionRabbitMQSetup, "setting up RabbitMQ exchanges and queues")

	// Setup dead letter exchange for storing unprocessable messages
	if err := setupDeadLetterExchange(ctx, ch, log); err != nil {
		return err
	}

	// Setup orders exchange (topic exchange)
	if err := setupOrdersExchange(ctx, ch, log); err != nil {
		return err
	}

	// Setup notifications exchange (fanout exchange)
	if err := setupNotificationsExchange(ctx, ch, log); err != nil {
		return err
	}

	// Setup kitchen queues with appropriate bindings
	if err := setupKitchenQueues(ctx, ch, log); err != nil {
		return err
	}

	// Setup notifications queue
	if err := setupNotificationsQueue(ctx, ch, log); err != nil {
		return err
	}

	log.Info(ctx, types.ActionRabbitMQSetupComplete, "RabbitMQ setup completed successfully")

	return nil
}

// setupDeadLetterExchange configures exchange and queue for "dead letters"
func setupDeadLetterExchange(ctx context.Context, ch *amqp091.Channel, log logger.Logger) error {
	// Declare exchange for "dead letters"
	err := ch.ExchangeDeclare(
		DeadLetterExchange, // name
		"fanout",           // type
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare dead letter exchange", err)
		return fmt.Errorf("failed to declare dead letter exchange: %w", err)
	}

	// Declare queue for "dead letters"
	_, err = ch.QueueDeclare(
		DeadLetterQueue, // name
		true,            // durable
		false,           // auto-deleted
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare dead letter queue", err)
		return fmt.Errorf("failed to declare dead letter queue: %w", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		DeadLetterQueue,    // queue name
		"",                 // routing key (irrelevant for fanout)
		DeadLetterExchange, // exchange name
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to bind dead letter queue", err)
		return fmt.Errorf("failed to bind dead letter queue: %w", err)
	}

	return nil
}

// setupOrdersExchange configures the exchange for orders
func setupOrdersExchange(ctx context.Context, ch *amqp091.Channel, log logger.Logger) error {
	err := ch.ExchangeDeclare(
		OrdersTopicExchange, // name
		"topic",             // type
		true,                // durable
		false,               // auto-deleted
		false,               // internal
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare orders topic exchange", err)
		return fmt.Errorf("failed to declare orders topic exchange: %w", err)
	}

	return nil
}

// setupNotificationsExchange configures the exchange for notifications
func setupNotificationsExchange(ctx context.Context, ch *amqp091.Channel, log logger.Logger) error {
	err := ch.ExchangeDeclare(
		NotificationsFanoutExchange, // name
		"fanout",                    // type
		true,                        // durable
		false,                       // auto-deleted
		false,                       // internal
		false,                       // no-wait
		nil,                         // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare notifications fanout exchange", err)
		return fmt.Errorf("failed to declare notifications fanout exchange: %w", err)
	}

	return nil
}

// setupKitchenQueues configures queues for the kitchen
func setupKitchenQueues(ctx context.Context, ch *amqp091.Channel, log logger.Logger) error {
	// Arguments for Dead Letter Exchange
	args := amqp091.Table{
		"x-dead-letter-exchange": DeadLetterExchange,
	}

	// Main queue for all orders
	_, err := ch.QueueDeclare(
		KitchenQueue, // name
		true,         // durable
		false,        // auto-deleted
		false,        // exclusive
		false,        // no-wait
		args,         // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare kitchen queue", err)
		return fmt.Errorf("failed to declare kitchen queue: %w", err)
	}

	// Bind main queue to all orders
	err = ch.QueueBind(
		KitchenQueue,        // queue name
		"kitchen.#",         // routing key (all orders)
		OrdersTopicExchange, // exchange name
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to bind kitchen queue", err)
		return fmt.Errorf("failed to bind kitchen queue: %w", err)
	}

	// Queue for "dine_in" orders
	_, err = ch.QueueDeclare(
		KitchenDineInQueue, // name
		true,               // durable
		false,              // auto-deleted
		false,              // exclusive
		false,              // no-wait
		args,               // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare kitchen dine_in queue", err)
		return fmt.Errorf("failed to declare kitchen dine_in queue: %w", err)
	}

	// Bind "dine_in" queue to corresponding orders
	err = ch.QueueBind(
		KitchenDineInQueue,  // queue name
		"kitchen.dine_in.#", // routing key
		OrdersTopicExchange, // exchange name
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to bind kitchen dine_in queue", err)
		return fmt.Errorf("failed to bind kitchen dine_in queue: %w", err)
	}

	// Queue for "takeout" orders
	_, err = ch.QueueDeclare(
		KitchenTakeoutQueue, // name
		true,                // durable
		false,               // auto-deleted
		false,               // exclusive
		false,               // no-wait
		args,                // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare kitchen takeout queue", err)
		return fmt.Errorf("failed to declare kitchen takeout queue: %w", err)
	}

	// Bind "takeout" queue to corresponding orders
	err = ch.QueueBind(
		KitchenTakeoutQueue, // queue name
		"kitchen.takeout.#", // routing key
		OrdersTopicExchange, // exchange name
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to bind kitchen takeout queue", err)
		return fmt.Errorf("failed to bind kitchen takeout queue: %w", err)
	}

	// Queue for "delivery" orders
	_, err = ch.QueueDeclare(
		KitchenDeliveryQueue, // name
		true,                 // durable
		false,                // auto-deleted
		false,                // exclusive
		false,                // no-wait
		args,                 // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare kitchen delivery queue", err)
		return fmt.Errorf("failed to declare kitchen delivery queue: %w", err)
	}

	// Bind "delivery" queue to corresponding orders
	err = ch.QueueBind(
		KitchenDeliveryQueue, // queue name
		"kitchen.delivery.#", // routing key
		OrdersTopicExchange,  // exchange name
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to bind kitchen delivery queue", err)
		return fmt.Errorf("failed to bind kitchen delivery queue: %w", err)
	}

	return nil
}

// setupNotificationsQueue configures the queue for notifications
func setupNotificationsQueue(ctx context.Context, ch *amqp091.Channel, log logger.Logger) error {
	// Declare queue for notifications
	_, err := ch.QueueDeclare(
		NotificationsQueue, // name
		true,               // durable
		false,              // auto-deleted
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to declare notifications queue", err)
		return fmt.Errorf("failed to declare notifications queue: %w", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		NotificationsQueue,          // queue name
		"",                          // routing key (irrelevant for fanout)
		NotificationsFanoutExchange, // exchange name
		false,                       // no-wait
		nil,                         // arguments
	)
	if err != nil {
		log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to bind notifications queue", err)
		return fmt.Errorf("failed to bind notifications queue: %w", err)
	}

	return nil
}
