package port

import (
	"context"
	"time"
	"wheres-my-pizza/internal/core/domain/models"
)

type OrderService interface {
	CreatNewOrder(ctx context.Context, newOrder models.CreateOrder) (models.OrderResponse, error)
	CheckConcurrent(ctx context.Context, limit int) bool
}

type OrderRepository interface {
	CreateNewOrder(ctx context.Context, newOrder models.CreateOrder) (string, time.Time, error)
	GetNumberOrdersProcessed(ctx context.Context) (int, error)
}

type RabbitMQ interface{}

type OrderAPI interface {
	Run(ctx context.Context)
}
