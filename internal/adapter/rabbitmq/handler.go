package rabbitmq

import (
	"context"
	"github.com/rabbitmq/amqp091-go"
)

// MessageHandler defines an interface for message handlers
type MessageHandler interface {
	HandleMessage(ctx context.Context, delivery amqp091.Delivery) error
}

type MessageHandlerFunc func(ctx context.Context, delivery amqp091.Delivery) error

// HandleMessage implements the MessageHandler interface
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, delivery amqp091.Delivery) error {
	return f(ctx, delivery)
}
