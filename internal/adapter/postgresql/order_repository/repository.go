package order_repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
	"wheres-my-pizza/internal/adapter/postgresql"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/pkg/config"
)

type OrderRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

func NewOrderRepository(ctx context.Context, cfg config.Config) (*OrderRepository, error) {
	pool, err := pgxpool.New(ctx, postgresql.BuildDSN(cfg))
	if err != nil {
		return &OrderRepository{}, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		return &OrderRepository{}, err
	}

	return &OrderRepository{
		pool: pool,
	}, nil
}

func (repo *OrderRepository) CreateNewOrder(ctx context.Context, newOrder models.CreateOrder) (string, time.Time, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}

	// Добавляем defer для rollback при ошибке
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// В запросе не указываем поле number, чтобы сработал триггер
	query := `
INSERT INTO orders (
    customer_name,
    type,
    table_number,
    delivery_address,
    total_amount,
    priority,
    status
) VALUES ($1, $2, $3, $4, $5, COALESCE($6, 1), 'received')
RETURNING id, number, created_at
`

	var id int
	var number string
	var createdAt time.Time
	err = tx.QueryRow(
		ctx,
		query,
		newOrder.CustomerName,
		newOrder.OrderType,
		newOrder.TableNumber,
		newOrder.DeliveryAddress,
		newOrder.TotalPrice,
		newOrder.Priority,
	).Scan(&id, &number, &createdAt)

	if err != nil {
		return "", time.Time{}, fmt.Errorf("ошибка при создании заказа: %w", err)
	}

	// Для отладки
	fmt.Printf("Создан заказ - id: %d, number: %s, time: %s\n", id, number, createdAt.Format(time.RFC3339))

	// Добавление элементов заказа
	itemsQuery := `
INSERT INTO order_items (
    order_id,
    name,
    quantity,
    price
) VALUES ($1, $2, $3, $4)
`

	for _, item := range newOrder.OrderItems {
		_, err = tx.Exec(
			ctx,
			itemsQuery,
			id,
			item.Name,
			item.Quantity,
			item.Price,
		)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("ошибка при добавлении элементов заказа: %w", err)
		}
	}

	// Добавление записи в лог статусов
	statusQuery := `
INSERT INTO order_status_log (
    order_id,
    status,
    changed_by
) VALUES ($1, $2, $3)
`
	_, err = tx.Exec(
		ctx,
		statusQuery,
		id,
		"received",
		"order-service",
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("ошибка при добавлении записи статуса: %w", err)
	}

	// Коммит транзакции
	err = tx.Commit(ctx)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("ошибка при завершении транзакции: %w", err)
	}

	return number, createdAt, nil
}
func (repo *OrderRepository) GetNumberOrdersProcessed(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) AS active_orders
FROM orders
WHERE status IN ('received', 'cooking', 'ready');
`
	count := 0
	err := repo.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (repo *OrderRepository) Close() {
	repo.pool.Close()
}
