package order_producer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"time"
	"wheres-my-pizza/internal/adapter/rabbitmq"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

// OrderProducer handles publishing orders to RabbitMQ
type OrderProducer struct {
	conn *rabbitmq.Connection
	log  logger.Logger
}

// NewOrderProducer creates a new producer for sending orders
func NewOrderProducer(ctx context.Context, cfg config.Config) (*OrderProducer, error) {
	log := logger.InitLogger("order_producer", logger.LevelDebug)

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

	return &OrderProducer{
		conn: conn,
		log:  log,
	}, nil
}

// PublishOrder publishes an order to the RabbitMQ queue
func (p *OrderProducer) PublishOrder(ctx context.Context, order models.CreateOrder, orderNumber string) error {
	// Create message for publication
	message := struct {
		OrderNumber     string             `json:"order_number"`
		CustomerName    string             `json:"customer_name"`
		OrderType       string             `json:"order_type"`
		TableNumber     *int               `json:"table_number,omitempty"`
		DeliveryAddress *string            `json:"delivery_address,omitempty"`
		Items           []models.OrderItem `json:"items"`
		TotalAmount     float64            `json:"total_amount"`
		Priority        int                `json:"priority"`
	}{
		OrderNumber:     orderNumber,
		CustomerName:    order.CustomerName,
		OrderType:       order.OrderType,
		TableNumber:     order.TableNumber,
		DeliveryAddress: order.DeliveryAddress,
		Items:           order.OrderItems,
		TotalAmount:     order.TotalPrice,
		Priority:        order.Priority,
	}

	// Marshal message to JSON
	jsonBody, err := json.Marshal(message)
	if err != nil {
		p.log.Error(ctx, types.ActionRabbitmqPublishFailed, "failed to marshal order message", err)
		return fmt.Errorf("failed to marshal order message: %w", err)
	}

	// Create routing key based on order type and priority
	routingKey := fmt.Sprintf("kitchen.%s.%d", order.OrderType, order.Priority)

	// Set up publication parameters
	err = p.conn.PublishWithContext(
		ctx,
		rabbitmq.OrdersTopicExchange, // exchange name
		routingKey,                   // routing key
		false,                        // mandatory
		false,                        // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, // message persisted to disk
			Priority:     uint8(order.Priority),
			Timestamp:    time.Now(),
			Body:         jsonBody,
		},
	)

	if err != nil {
		p.log.Error(ctx, types.ActionRabbitmqPublishFailed, "failed to publish order message", err)
		return fmt.Errorf("failed to publish order message: %w", err)
	}

	p.log.Debug(ctx, types.ActionOrderPublished, "order message published successfully",
		"order_number", orderNumber,
		"routing_key", routingKey,
	)

	return nil
}

// PublishStatusUpdate publishes an order status update
// Добавлен новый метод для реализации интерфейса port.RabbitMQ
func (p *OrderProducer) PublishStatusUpdate(ctx context.Context, statusUpdate models.StatusUpdate) error {
	jsonBody, err := json.Marshal(statusUpdate)
	if err != nil {
		p.log.Error(ctx, types.ActionRabbitmqPublishFailed, "failed to marshal status update", err)
		return fmt.Errorf("failed to marshal status update: %w", err)
	}

	// Publish to notifications exchange (fanout)
	err = p.conn.PublishWithContext(
		ctx,
		rabbitmq.NotificationsFanoutExchange, // exchange name
		"",                                   // routing key (not used for fanout)
		false,                                // mandatory
		false,                                // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, // message persisted to disk
			Timestamp:    time.Now(),
			Body:         jsonBody,
		},
	)

	if err != nil {
		p.log.Error(ctx, types.ActionRabbitmqPublishFailed, "failed to publish status update", err)
		return fmt.Errorf("failed to publish status update: %w", err)
	}

	p.log.Debug(ctx, types.ActionStatusUpdatePublished, "status update published successfully",
		"order_number", statusUpdate.OrderNumber,
		"new_status", statusUpdate.NewStatus,
	)

	return nil
}

// Close closes the connection to RabbitMQ
func (p *OrderProducer) Close() error {
	return p.conn.Close()
}
