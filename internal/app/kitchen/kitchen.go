package kitchen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rabbitmq/amqp091-go"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"wheres-my-pizza/internal/adapter/postgresql"
	"wheres-my-pizza/internal/adapter/postgresql/worker_repository"
	"wheres-my-pizza/internal/adapter/rabbitmq/kitchen_worker"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/flags"
	"wheres-my-pizza/pkg/logger"
)

// KitchenApp represents the kitchen worker application
type KitchenApp struct {
	ctx        context.Context
	cancel     context.CancelFunc
	logger     logger.Logger
	worker     *kitchen_worker.KitchenWorker
	repo       *worker_repository.WorkerRepository
	workerName string
	orderTypes []string
	heartbeat  time.Duration
}

// NewKitchenApp creates a new kitchen application
func NewKitchenApp() *KitchenApp {
	cfg, err := config.ParseYAML()
	if err != nil {
		config.PrintYAMLHelp()
		slog.Error("failed to configure application", "error", err)
		os.Exit(1)
	}

	logger := logger.InitLogger("Kitchen Worker", logger.LevelDebug)

	// Check required worker name
	if *flags.WorkerName == "" {
		logger.Error(context.Background(), types.ActionWorkerRegistrationFailed,
			"worker name is required",
			fmt.Errorf("worker name is required"),
		)
		os.Exit(1)
	}

	// Process order types
	var orderTypes []string
	if *flags.OrderTypes != "" {
		orderTypes = strings.Split(*flags.OrderTypes, ",")
		for i := range orderTypes {
			orderTypes[i] = strings.TrimSpace(orderTypes[i])
		}

		// Validate order types
		for _, orderType := range orderTypes {
			if orderType != "dine_in" && orderType != "takeout" && orderType != "delivery" {
				logger.Error(context.Background(), types.ActionWorkerRegistrationFailed,
					"invalid order type",
					fmt.Errorf("invalid order type: %s", orderType),
				)
				os.Exit(1)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Try to create workers table if it doesn't exist
	pool, err := pgxpool.New(ctx, postgresql.BuildDSN(cfg))
	if err == nil {
		_, err = pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS workers (
				id                SERIAL PRIMARY KEY,
				created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
				name              TEXT UNIQUE NOT NULL,
				type              TEXT NOT NULL,
				status            TEXT DEFAULT 'online',
				last_seen         TIMESTAMPTZ DEFAULT current_timestamp,
				orders_processed  INTEGER DEFAULT 0
			);
			
			CREATE INDEX IF NOT EXISTS idx_workers_status ON workers(status);
			CREATE INDEX IF NOT EXISTS idx_workers_last_seen ON workers(last_seen);
		`)
		if err != nil {
			logger.Error(ctx, "create_workers_table_failed", "failed to create workers table", err)
		}
		pool.Close()
	}

	// Connect to database for worker registration
	repo, err := worker_repository.NewWorkerRepository(ctx, cfg)
	if err != nil {
		cancel()
		logger.Error(ctx, types.ActionDBConnectFailed, "failed to connect to database", err)
		os.Exit(1)
	}

	// Register worker or update existing one
	err = repo.RegisterWorker(ctx, *flags.WorkerName, strings.Join(orderTypes, ","))
	if err != nil {
		cancel()
		repo.Close()
		logger.Error(ctx, types.ActionWorkerRegistrationFailed, "failed to register worker", err)
		os.Exit(1)
	}

	logger.Info(ctx, types.ActionWorkerRegistered, "worker registered successfully",
		"worker_name", *flags.WorkerName,
		"order_types", orderTypes,
	)

	// Connect to RabbitMQ
	worker, err := kitchen_worker.NewKitchenWorker(ctx, cfg, *flags.WorkerName, orderTypes, *flags.Prefetch)
	if err != nil {
		cancel()
		repo.Close()
		logger.Error(ctx, types.ActionRabbitMQConnectFailed, "failed to connect to RabbitMQ", err)
		os.Exit(1)
	}

	return &KitchenApp{
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		worker:     worker,
		repo:       repo,
		workerName: *flags.WorkerName,
		orderTypes: orderTypes,
		heartbeat:  time.Duration(*flags.HeartbeatInterval) * time.Second,
	}
}

// Start begins the kitchen worker operation
func (app *KitchenApp) Start() {
	// Start goroutine for sending heartbeats
	go app.sendHeartbeats()

	// Start message processing
	go app.consumeOrders()

	// Wait for signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	app.logger.Info(app.ctx, types.ActionGracefulShutdown, "service is shutting down")
	app.cancel()

	// Update worker status to "offline"
	if err := app.repo.UpdateWorkerStatus(context.Background(), app.workerName, "offline"); err != nil {
		app.logger.Error(app.ctx, types.ActionGracefulShutdown, "error updating worker status", err)
	}

	// Close connections
	if app.worker != nil {
		if err := app.worker.Close(); err != nil {
			app.logger.Error(app.ctx, types.ActionGracefulShutdown, "error closing RabbitMQ connection", err)
		}
	}

	if app.repo != nil {
		app.repo.Close()
	}
}

// sendHeartbeats sends periodic signals to update worker status
func (app *KitchenApp) sendHeartbeats() {
	ticker := time.NewTicker(app.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			if err := app.repo.UpdateWorkerStatus(app.ctx, app.workerName, "online"); err != nil {
				app.logger.Error(app.ctx, types.ActionHeartbeatSent, "error sending heartbeat", err)
			} else {
				app.logger.Debug(app.ctx, types.ActionHeartbeatSent, "heartbeat sent successfully")
			}
		}
	}
}

// consumeOrders starts processing orders from the queue
func (app *KitchenApp) consumeOrders() {
	err := app.worker.ConsumeOrders(app.ctx, func(delivery amqp091.Delivery) error {
		var rawData map[string]interface{}
		if err := json.Unmarshal(delivery.Body, &rawData); err != nil {
			fmt.Printf("DEBUG: Error parsing JSON for debugging: %v\n", err)
		} else {
			orderNum, ok := rawData["order_number"]
			if !ok {
				fmt.Println("DEBUG: Attention! The order_number field is missing in the JSON of the message!")
			} else {
				fmt.Printf("DEBUG: Поле order_number в сообщении = %v (тип: %T)\n", orderNum, orderNum)
			}
		}

		var order struct {
			OrderNumber     string             `json:"order_number"`
			CustomerName    string             `json:"customer_name"`
			OrderType       string             `json:"order_type"`
			TableNumber     *int               `json:"table_number,omitempty"`
			DeliveryAddress *string            `json:"delivery_address,omitempty"`
			Items           []models.OrderItem `json:"items"`
			TotalAmount     float64            `json:"total_amount"`
			Priority        int                `json:"priority"`
		}

		if err := json.Unmarshal(delivery.Body, &order); err != nil {
			fmt.Printf("DEBUG: Error parsing JSON into the order structure: %v\n", err)
			app.logger.Error(app.ctx, types.ActionMessageProcessingFailed, "failed to unmarshal order", err)
			return err
		}

		fmt.Printf("DEBUG: Disassembled order data - OrderNumber: '%s', CustomerName: '%s', OrderType: '%s', TotalAmount: %.2f\n",
			order.OrderNumber, order.CustomerName, order.OrderType, order.TotalAmount)

		app.logger.Debug(app.ctx, types.ActionOrderProcessingStarted, "processing order",
			"order_number", order.OrderNumber,
			"order_type", order.OrderType,
		)

		// Проверка пустого номера заказа
		if order.OrderNumber == "" {
			fmt.Println("DEBUG: CRITICAL ERROR - the order number is empty!")
			app.logger.Error(app.ctx, "empty_order_number", "order number is empty",
				errors.New("empty order number"))
			return errors.New("empty order number")
		}

		// Check if this order is already being processed (idempotency)
		if app.worker.IsInProgress(order.OrderNumber) {
			app.logger.Info(app.ctx, types.ActionOrderRejected, "order is already being processed",
				"order_number", order.OrderNumber,
			)
			return nil // Just acknowledge message without processing
		}

		// Mark order as being processed
		app.worker.MarkInProgress(order.OrderNumber)
		defer app.worker.UnmarkInProgress(order.OrderNumber)

		// Check if context is already canceled
		if app.ctx.Err() != nil {
			return fmt.Errorf("context already canceled: %w", app.ctx.Err())
		}

		// Create a separate context for DB operations that's not tied to app.ctx
		// This ensures DB operations can complete even if app.ctx is cancelled
		opCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Step 1: Update status to "cooking"
		fmt.Printf("DEBUG: Attempt to update the %s order status on cooking\n", order.OrderNumber)

		currentStatus, err := app.repo.UpdateOrderStatus(opCtx, order.OrderNumber, "received", "cooking", app.workerName)
		if err != nil {
			fmt.Printf("DEBUG: Ошибка при обновлении статуса на cooking: %v\n", err)
			app.logger.Error(app.ctx, types.ActionDBTransactionFailed, "failed to update order status to cooking", err)

			// Проверка существования заказа напрямую
			exists, err := app.repo.CheckOrderExists(opCtx, order.OrderNumber)
			if err != nil {
				fmt.Printf("DEBUG: Ошибка при проверке существования заказа: %v\n", err)
			} else {
				fmt.Printf("DEBUG: Проверка существования заказа %s: %t\n", order.OrderNumber, exists)
			}

			return models.ErrorDbTransactionFailed
		}

		fmt.Printf("DEBUG: Успешно обновлен статус заказа %s на cooking, предыдущий статус: %s\n",
			order.OrderNumber, currentStatus)

		// Send notification about status update
		estimatedCompletion := time.Now().Add(getCookingTime(order.OrderType))

		statusUpdate := models.StatusUpdate{
			OrderNumber:         order.OrderNumber,
			OldStatus:           "received",
			NewStatus:           "cooking",
			ChangedBy:           app.workerName,
			Timestamp:           time.Now(),
			EstimatedCompletion: estimatedCompletion,
		}

		// Use separate context for publishing too
		if err = app.worker.PublishStatusUpdate(opCtx, statusUpdate); err != nil {
			fmt.Printf("DEBUG: Ошибка при публикации обновления статуса: %v\n", err)
			app.logger.Error(app.ctx, types.ActionRabbitmqPublishFailed, "failed to publish status update", err)
			return models.ErrorRabbitmqPublishFailed
		}

		fmt.Printf("DEBUG: Успешно опубликовано обновление статуса заказа %s на cooking\n", order.OrderNumber)

		// Simulate cooking process with a separate timer to avoid context cancellation issues
		cookingTime := getCookingTime(order.OrderType)
		fmt.Printf("DEBUG: Начинаем симуляцию готовки заказа %s, время готовки: %v\n",
			order.OrderNumber, cookingTime)

		cookingDone := make(chan struct{})
		go func() {
			timer := time.NewTimer(cookingTime)
			defer timer.Stop()
			select {
			case <-timer.C:
				fmt.Printf("DEBUG: Готовка заказа %s завершена\n", order.OrderNumber)
				close(cookingDone)
			case <-app.ctx.Done():
				fmt.Printf("DEBUG: Готовка заказа %s прервана отменой контекста\n", order.OrderNumber)
				// Let the main goroutine handle cancellation
			}
		}()

		// Wait for cooking to finish or context to be canceled
		select {
		case <-app.ctx.Done():
			fmt.Printf("DEBUG: Обработка заказа %s прервана отменой контекста\n", order.OrderNumber)
			app.logger.Info(app.ctx, types.ActionGracefulShutdown, "order processing interrupted due to shutdown",
				"order_number", order.OrderNumber,
			)
			return fmt.Errorf("interrupted")
		case <-cookingDone:
			fmt.Printf("DEBUG: Продолжаем обработку заказа %s после завершения готовки\n", order.OrderNumber)
			// Continue processing
		}

		// Check context again before final operations
		if app.ctx.Err() != nil {
			fmt.Printf("DEBUG: Контекст отменен во время готовки заказа %s\n", order.OrderNumber)
			return fmt.Errorf("context canceled during cooking: %w", app.ctx.Err())
		}

		// Step 2: Update status to "ready" - use a fresh context
		opCtx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()

		fmt.Printf("DEBUG: Попытка обновить статус заказа %s на ready\n", order.OrderNumber)
		_, err = app.repo.UpdateOrderStatus(opCtx2, order.OrderNumber, "cooking", "ready", app.workerName)
		if err != nil {
			fmt.Printf("DEBUG: Ошибка при обновлении статуса на ready: %v\n", err)
			app.logger.Error(app.ctx, types.ActionDBTransactionFailed, "failed to update order status to ready", err)
			return models.ErrorDbTransactionFailed
		}

		fmt.Printf("DEBUG: Успешно обновлен статус заказа %s на ready\n", order.OrderNumber)

		// Increment processed orders counter
		if err := app.repo.IncrementOrdersProcessed(opCtx2, app.workerName); err != nil {
			fmt.Printf("DEBUG: Ошибка при увеличении счетчика обработанных заказов: %v\n", err)
			app.logger.Error(app.ctx, types.ActionDBTransactionFailed, "failed to increment orders processed", err)
			// Not critical error, continue
		} else {
			fmt.Printf("DEBUG: Успешно увеличен счетчик обработанных заказов для %s\n", app.workerName)
		}

		// Send notification about order readiness
		statusUpdate = models.StatusUpdate{
			OrderNumber: order.OrderNumber,
			OldStatus:   "cooking",
			NewStatus:   "ready",
			ChangedBy:   app.workerName,
			Timestamp:   time.Now(),
		}

		fmt.Printf("DEBUG: Отправляем уведомление о готовности заказа %s\n", order.OrderNumber)
		if err := app.worker.PublishStatusUpdate(opCtx2, statusUpdate); err != nil {
			fmt.Printf("DEBUG: Ошибка при публикации уведомления о готовности: %v\n", err)
			app.logger.Error(app.ctx, types.ActionRabbitmqPublishFailed, "failed to publish status update", err)
			return models.ErrorRabbitmqPublishFailed
		}

		fmt.Printf("DEBUG: Заказ %s успешно обработан\n", order.OrderNumber)
		app.logger.Debug(app.ctx, types.ActionOrderCompleted, "order processing completed",
			"order_number", order.OrderNumber,
		)

		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		app.logger.Error(app.ctx, types.ActionRabbitMQConsumeFailed, "error consuming orders", err)
		app.cancel() // Initiate application shutdown
	}
}

// getCookingTime returns cooking time based on order type
func getCookingTime(orderType string) time.Duration {
	switch orderType {
	case "dine_in":
		return 8 * time.Second
	case "takeout":
		return 10 * time.Second
	case "delivery":
		return 12 * time.Second
	default:
		return 10 * time.Second
	}
}
