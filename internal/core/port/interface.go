package port

import (
	"context"
	"time"
	"wheres-my-pizza/internal/core/domain/models"
)

// OrderService interface
type OrderService interface {
	CreatNewOrder(ctx context.Context, newOrder models.CreateOrder) (models.OrderResponse, error)
	CheckConcurrent(ctx context.Context, limit int) bool
}

// OrderRepository interface
type OrderRepository interface {
	CreateNewOrder(ctx context.Context, newOrder models.CreateOrder) (string, time.Time, error)
	GetNumberOrdersProcessed(ctx context.Context) (int, error)
}

// RabbitMQ interface
type RabbitMQ interface {
	PublishOrder(ctx context.Context, order models.CreateOrder, orderNumber string) error
	PublishStatusUpdate(ctx context.Context, statusUpdate models.StatusUpdate) error
	Close() error
}

// OrderAPI interface
type OrderAPI interface {
	Run(ctx context.Context)
}
