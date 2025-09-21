package tracking_repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
	"wheres-my-pizza/internal/adapter/postgresql"
	"wheres-my-pizza/pkg/config"
)

// TrackingRepository handles database operations for order and worker tracking
type TrackingRepository struct {
	pool *pgxpool.Pool
}

// NewTrackingRepository creates a new tracking repository
func NewTrackingRepository(ctx context.Context, cfg config.Config) (*TrackingRepository, error) {
	pool, err := pgxpool.New(ctx, postgresql.BuildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &TrackingRepository{
		pool: pool,
	}, nil
}

// GetOrderStatus returns the current status of an order
func (r *TrackingRepository) GetOrderStatus(ctx context.Context, orderNumber string) (map[string]interface{}, error) {
	query := `
		SELECT
			o.number as order_number,
			o.status as current_status,
			o.updated_at,
			o.processed_by,
			o.completed_at
		FROM
			orders o
		WHERE
			o.number = $1
	`

	row := r.pool.QueryRow(ctx, query, orderNumber)

	var (
		currentStatus string
		updatedAt     time.Time
		processedBy   *string
		completedAt   *time.Time
	)

	err := row.Scan(
		&orderNumber,
		&currentStatus,
		&updatedAt,
		&processedBy,
		&completedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("order not found")
		}
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	// Format response as map for easy JSON marshalling
	result := map[string]interface{}{
		"order_number":   orderNumber,
		"current_status": currentStatus,
		"updated_at":     updatedAt,
	}

	if processedBy != nil {
		result["processed_by"] = *processedBy
	}

	if completedAt != nil {
		result["completed_at"] = *completedAt
	}

	// Calculate estimated completion time if order is cooking
	if currentStatus == "cooking" && updatedAt.After(time.Now().Add(-30*time.Minute)) {
		// Get order type to determine cooking time
		var orderType string
		err = r.pool.QueryRow(ctx,
			`SELECT type FROM orders WHERE number = $1`,
			orderNumber,
		).Scan(&orderType)

		if err == nil {
			// Calculate based on order type
			var cookingTime time.Duration
			switch orderType {
			case "dine_in":
				cookingTime = 8 * time.Second
			case "takeout":
				cookingTime = 10 * time.Second
			case "delivery":
				cookingTime = 12 * time.Second
			default:
				cookingTime = 10 * time.Second
			}

			estimatedCompletion := updatedAt.Add(cookingTime)
			result["estimated_completion"] = estimatedCompletion
		}
	}

	return result, nil
}

// GetOrderHistory returns the status history of an order
func (r *TrackingRepository) GetOrderHistory(ctx context.Context, orderNumber string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			osl.status,
			osl.changed_at as timestamp,
			osl.changed_by
		FROM
			orders o
		JOIN
			order_status_log osl ON o.id = osl.order_id
		WHERE
			o.number = $1
		ORDER BY
			osl.changed_at ASC
	`

	rows, err := r.pool.Query(ctx, query, orderNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get order history: %w", err)
	}
	defer rows.Close()

	var history []map[string]interface{}

	for rows.Next() {
		var (
			status    string
			timestamp time.Time
			changedBy *string
		)

		err := rows.Scan(&status, &timestamp, &changedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order history row: %w", err)
		}

		entry := map[string]interface{}{
			"status":    status,
			"timestamp": timestamp,
		}

		if changedBy != nil {
			entry["changed_by"] = *changedBy
		} else {
			entry["changed_by"] = "system"
		}

		history = append(history, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order history rows: %w", err)
	}

	if len(history) == 0 {
		return nil, fmt.Errorf("order not found or has no history")
	}

	return history, nil
}

// GetWorkersStatus returns the status of all kitchen workers
func (r *TrackingRepository) GetWorkersStatus(ctx context.Context, heartbeatThreshold time.Duration) ([]map[string]interface{}, error) {
	query := `
		SELECT
			name as worker_name,
			status,
			orders_processed,
			last_seen,
			type as worker_type
		FROM
			workers
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get workers status: %w", err)
	}
	defer rows.Close()

	var workers []map[string]interface{}
	currentTime := time.Now()

	for rows.Next() {
		var (
			workerName      string
			status          string
			ordersProcessed int
			lastSeen        time.Time
			workerType      string
		)

		err := rows.Scan(&workerName, &status, &ordersProcessed, &lastSeen, &workerType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan worker status row: %w", err)
		}

		// If worker hasn't updated status longer than threshold, mark as offline
		if status == "online" && currentTime.Sub(lastSeen) > heartbeatThreshold {
			status = "offline"
		}

		worker := map[string]interface{}{
			"worker_name":      workerName,
			"status":           status,
			"orders_processed": ordersProcessed,
			"last_seen":        lastSeen,
		}

		// Include worker type if available
		if workerType != "" {
			worker["worker_type"] = workerType
		}

		workers = append(workers, worker)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating worker status rows: %w", err)
	}

	return workers, nil
}

// Close closes the database connection
func (r *TrackingRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}
