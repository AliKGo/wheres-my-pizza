// internal/app/order/order.go
package order

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"wheres-my-pizza/internal/adapter/http"
	"wheres-my-pizza/internal/adapter/postgresql/order_repository"
	"wheres-my-pizza/internal/adapter/rabbitmq/order_producer"
	"wheres-my-pizza/internal/adapter/server"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/internal/core/serivce/order"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

// OrderApp represents the order service application
type OrderApp struct {
	api    *server.API
	ctx    context.Context
	cancel context.CancelFunc
	logger logger.Logger
	rabbit *order_producer.OrderProducer
	repo   *order_repository.OrderRepository
}

// NewOrderApp creates a new order service application
func NewOrderApp() *OrderApp {
	cfg, err := config.ParseYAML()
	if err != nil {
		config.PrintYAMLHelp()
		slog.Error("failed to configure application", "error", err)
		os.Exit(1)
	}

	logger := logger.InitLogger("Order Service", logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())

	// Connect to database
	repo, err := order_repository.NewOrderRepository(ctx, cfg)
	if err != nil {
		cancel()
		config.PrintYAMLHelp()
		slog.Error("failed to configure database", "error", err)
		os.Exit(1)
	}

	// Connect to RabbitMQ
	rabbit, err := order_producer.NewOrderProducer(ctx, cfg)
	if err != nil {
		cancel()
		repo.Close()
		config.PrintYAMLHelp()
		slog.Error("failed to configure RabbitMQ", "error", err)
		os.Exit(1)
	}

	// Исправлено: передаем оба аргумента
	svc := order.NewOrderService(repo, rabbit)
	handle := http.NewOrderHandle(ctx, svc)

	api := server.NewRouter(logger, handle)

	return &OrderApp{
		api:    api,
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
		rabbit: rabbit,
		repo:   repo,
	}
}

// Start begins the order service operation
func (app *OrderApp) Start() {
	app.api.Run(app.ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	app.cancel()
	app.logger.Info(app.ctx, types.ActionGracefulShutdown, "service is shutting down")

	// Close RabbitMQ connection
	if app.rabbit != nil {
		if err := app.rabbit.Close(); err != nil {
			app.logger.Error(app.ctx, types.ActionGracefulShutdown, "error closing RabbitMQ connection", err)
		}
	}

	// Close database connection
	if app.repo != nil {
		app.repo.Close()
	}
}
