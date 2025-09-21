package kitchen_worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"time"
	"wheres-my-pizza/internal/adapter/rabbitmq"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

// KitchenWorker handles processing orders from the kitchen queue
type KitchenWorker struct {
	conn       *rabbitmq.Connection
	log        logger.Logger
	workerName string
	orderTypes []string
	queueName  string
	prefetch   int
	inProgress map[string]struct{} // for tracking orders being processed (idempotency)
}

// NewKitchenWorker creates a new kitchen worker
func NewKitchenWorker(ctx context.Context, cfg config.Config, workerName string, orderTypes []string, prefetch int) (*KitchenWorker, error) {
	log := logger.InitLogger(fmt.Sprintf("kitchen_worker_%s", workerName), logger.LevelDebug)

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

	// Determine appropriate queue based on order types
	queueName := rabbitmq.KitchenQueue // default is the general queue

	if len(orderTypes) == 1 {
		switch orderTypes[0] {
		case "dine_in":
			queueName = rabbitmq.KitchenDineInQueue
		case "takeout":
			queueName = rabbitmq.KitchenTakeoutQueue
		case "delivery":
			queueName = rabbitmq.KitchenDeliveryQueue
		}
	}

	return &KitchenWorker{
		conn:       conn,
		log:        log,
		workerName: workerName,
		orderTypes: orderTypes,
		queueName:  queueName,
		prefetch:   prefetch,
		inProgress: make(map[string]struct{}),
	}, nil
}

// ConsumeOrders starts consuming orders from the queue
func (w *KitchenWorker) ConsumeOrders(ctx context.Context, handler func(delivery amqp091.Delivery) error) error {
	ch := w.conn.Channel()

	// Set prefetch count
	if err := ch.Qos(w.prefetch, 0, false); err != nil {
		w.log.Error(ctx, types.ActionRabbitMQSetupFailed, "failed to set QoS", err)
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming from queue
	// internal/adapter/rabbitmq/kitchen_worker/worker.go (продолжение)
	// Start consuming from queue
	msgs, err := ch.Consume(
		w.queueName,  // queue name
		w.workerName, // consumer name (unique for each worker)
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		w.log.Error(ctx, types.ActionRabbitMQConsumeFailed, "failed to start consuming", err)
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	w.log.Info(ctx, types.ActionRabbitMQConsumeStarted, "started consuming from queue",
		"queue", w.queueName,
		"prefetch", w.prefetch,
	)

	for {
		select {
		case <-ctx.Done():
			w.log.Info(ctx, types.ActionGracefulShutdown, "stopping consumption due to context cancellation")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				w.log.Error(ctx, types.ActionRabbitMQConsumeFailed, "message channel closed", errors.New("channel closed"))
				return errors.New("message channel closed")
			}

			// Check order type if worker is specialized
			if len(w.orderTypes) > 0 {
				var orderMsg struct {
					OrderType string `json:"order_type"`
				}
				if err := json.Unmarshal(msg.Body, &orderMsg); err == nil {
					// Check if worker can handle this order type
					canProcess := false
					for _, t := range w.orderTypes {
						if t == orderMsg.OrderType {
							canProcess = true
							break
						}
					}

					if !canProcess {
						// Return order to queue for another worker
						w.log.Debug(ctx, types.ActionOrderRejected, "rejecting order of incompatible type",
							"order_type", orderMsg.OrderType,
							"supported_types", w.orderTypes,
						)
						if err := msg.Nack(false, true); err != nil {
							w.log.Error(ctx, types.ActionRabbitMQNackFailed, "failed to nack message", err)
						}
						continue
					}
				}
			}

			// Process the message using provided handler
			w.log.Debug(ctx, types.ActionOrderProcessingStarted, "processing order from queue")

			if err := handler(msg); err != nil {
				w.log.Error(ctx, types.ActionMessageProcessingFailed, "failed to process message", err)
				// Return message to queue (or send to DLQ depending on error)
				if errors.Is(err, models.ErrorDbTransactionFailed) || errors.Is(err, models.ErrorRabbitmqPublishFailed) {
					// Temporary error, return to queue
					if err := msg.Nack(false, true); err != nil {
						w.log.Error(ctx, types.ActionRabbitMQNackFailed, "failed to nack message", err)
					}
				} else {
					// Permanent error (e.g., validation), send to DLQ
					if err := msg.Nack(false, false); err != nil {
						w.log.Error(ctx, types.ActionRabbitMQNackFailed, "failed to nack message to DLQ", err)
					}
				}
				continue
			}

			// Successful processing, acknowledge message
			if err := msg.Ack(false); err != nil {
				w.log.Error(ctx, types.ActionRabbitMQAckFailed, "failed to ack message", err)
			}
		}
	}
}

// PublishStatusUpdate publishes an order status update
func (w *KitchenWorker) PublishStatusUpdate(ctx context.Context, statusUpdate models.StatusUpdate) error {
	jsonBody, err := json.Marshal(statusUpdate)
	if err != nil {
		w.log.Error(ctx, types.ActionRabbitmqPublishFailed, "failed to marshal status update", err)
		return fmt.Errorf("failed to marshal status update: %w", err)
	}

	// Publish to notifications exchange (fanout)
	err = w.conn.PublishWithContext(
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
		w.log.Error(ctx, types.ActionRabbitmqPublishFailed, "failed to publish status update", err)
		return fmt.Errorf("failed to publish status update: %w", err)
	}

	w.log.Debug(ctx, types.ActionStatusUpdatePublished, "status update published successfully",
		"order_number", statusUpdate.OrderNumber,
		"new_status", statusUpdate.NewStatus,
	)

	return nil
}

// IsInProgress checks if an order is currently being processed
func (w *KitchenWorker) IsInProgress(orderNumber string) bool {
	_, exists := w.inProgress[orderNumber]
	return exists
}

// MarkInProgress marks an order as being processed
func (w *KitchenWorker) MarkInProgress(orderNumber string) {
	w.inProgress[orderNumber] = struct{}{}
}

// UnmarkInProgress removes the processing mark from an order
func (w *KitchenWorker) UnmarkInProgress(orderNumber string) {
	delete(w.inProgress, orderNumber)
}

// Close closes the connection to RabbitMQ
func (w *KitchenWorker) Close() error {
	return w.conn.Close()
}
