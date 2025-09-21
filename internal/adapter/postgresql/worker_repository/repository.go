package worker_repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
	"wheres-my-pizza/internal/adapter/postgresql"
	"wheres-my-pizza/pkg/config"
)

// WorkerRepository handles database operations for kitchen workers
type WorkerRepository struct {
	pool *pgxpool.Pool
}

// NewWorkerRepository creates a new worker repository
func NewWorkerRepository(ctx context.Context, cfg config.Config) (*WorkerRepository, error) {
	pool, err := pgxpool.New(ctx, postgresql.BuildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &WorkerRepository{
		pool: pool,
	}, nil
}

// RegisterWorker registers a kitchen worker in the database
func (r *WorkerRepository) RegisterWorker(ctx context.Context, name string, orderTypes string) error {
	// Check if worker with this name already exists
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT status FROM workers WHERE name = $1`,
		name,
	).Scan(&status)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Worker does not exist, create new one
			_, err := r.pool.Exec(ctx,
				`INSERT INTO workers (name, type, status, last_seen) 
				 VALUES ($1, $2, $3, now())`,
				name, orderTypes, "online",
			)
			if err != nil {
				return fmt.Errorf("failed to insert worker: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to check worker existence: %w", err)
	}

	// Worker exists, check its status
	if status == "online" {
		return fmt.Errorf("worker with name '%s' is already online", name)
	}

	// Worker is offline, update its status
	_, err = r.pool.Exec(ctx,
		`UPDATE workers SET status = 'online', last_seen = now(), type = $2 WHERE name = $1`,
		name, orderTypes,
	)
	if err != nil {
		return fmt.Errorf("failed to update worker status: %w", err)
	}

	return nil
}

// UpdateWorkerStatus updates a kitchen worker's status
func (r *WorkerRepository) UpdateWorkerStatus(ctx context.Context, name string, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE workers SET status = $2, last_seen = now() WHERE name = $1`,
		name, status,
	)
	if err != nil {
		return fmt.Errorf("failed to update worker status: %w", err)
	}

	return nil
}

// UpdateOrderStatus updates an order's status and logs the change
func (r *WorkerRepository) UpdateOrderStatus(ctx context.Context, orderNumber string, expectedStatus string, newStatus string, changedBy string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// Get current status and order ID
	var currentStatus string
	var orderID int
	err = tx.QueryRow(ctx,
		`SELECT id, status FROM orders WHERE number = $1`,
		orderNumber,
	).Scan(&orderID, &currentStatus)
	if err != nil {
		return "", fmt.Errorf("failed to get order info: %w", err)
	}

	// Check if current status matches expected
	if expectedStatus != "" && currentStatus != expectedStatus {
		return currentStatus, fmt.Errorf("order status mismatch: expected %s, got %s", expectedStatus, currentStatus)
	}

	// Update status
	completedAt := sql.NullTime{}
	if newStatus == "ready" {
		completedAt.Time = time.Now()
		completedAt.Valid = true
	}

	_, err = tx.Exec(ctx,
		`UPDATE orders SET 
			status = $2,
			processed_by = $3,
			updated_at = now(),
			completed_at = CASE WHEN $4::boolean THEN now() ELSE completed_at END
		WHERE number = $1`,
		orderNumber, newStatus, changedBy, newStatus == "ready",
	)
	if err != nil {
		return "", fmt.Errorf("failed to update order status: %w", err)
	}

	// Log status change
	_, err = tx.Exec(ctx,
		`INSERT INTO order_status_log (order_id, status, changed_by, changed_at)
		 VALUES ($1, $2, $3, now())`,
		orderID, newStatus, changedBy,
	)
	if err != nil {
		return "", fmt.Errorf("failed to log status change: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return currentStatus, nil
}

// IncrementOrdersProcessed increments the worker's processed orders counter
func (r *WorkerRepository) IncrementOrdersProcessed(ctx context.Context, name string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE workers SET orders_processed = orders_processed + 1 WHERE name = $1`,
		name,
	)
	if err != nil {
		return fmt.Errorf("failed to increment orders processed: %w", err)
	}

	return nil
}

// Close closes the database connection
func (r *WorkerRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

// CheckOrderExists проверяет существование заказа с указанным номером
func (r *WorkerRepository) CheckOrderExists(ctx context.Context, orderNumber string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM orders WHERE number = $1)`,
		orderNumber,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check if order exists: %w", err)
	}

	return exists, nil
}
