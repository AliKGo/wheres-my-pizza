// internal/adapter/rabbitmq/metrics.go
package rabbitmq

import (
	"sync"
	"time"
)

// RabbitMQMetrics stores metrics for RabbitMQ operations
type RabbitMQMetrics struct {
	mu                    sync.Mutex
	messagesPublished     int64
	messagesConsumed      int64
	messagesFailed        int64
	lastReconnectAttempt  time.Time
	reconnectAttempts     int64
	lastSuccessfulConnect time.Time
}

// NewRabbitMQMetrics creates a new metrics instance
func NewRabbitMQMetrics() *RabbitMQMetrics {
	return &RabbitMQMetrics{
		lastSuccessfulConnect: time.Now(),
	}
}

// IncrementPublished increases the published messages counter
func (m *RabbitMQMetrics) IncrementPublished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesPublished++
}

// IncrementConsumed increases the consumed messages counter
func (m *RabbitMQMetrics) IncrementConsumed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesConsumed++
}

// IncrementFailed increases the failed messages counter
func (m *RabbitMQMetrics) IncrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesFailed++
}

// RecordReconnectAttempt records a reconnection attempt
func (m *RabbitMQMetrics) RecordReconnectAttempt() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastReconnectAttempt = time.Now()
	m.reconnectAttempts++
}

// RecordSuccessfulConnect records a successful connection
func (m *RabbitMQMetrics) RecordSuccessfulConnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSuccessfulConnect = time.Now()
}

// GetMetrics returns current metrics as a map
func (m *RabbitMQMetrics) GetMetrics() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	return map[string]interface{}{
		"messages_published":      m.messagesPublished,
		"messages_consumed":       m.messagesConsumed,
		"messages_failed":         m.messagesFailed,
		"reconnect_attempts":      m.reconnectAttempts,
		"last_reconnect_attempt":  m.lastReconnectAttempt,
		"last_successful_connect": m.lastSuccessfulConnect,
		"uptime_seconds":          time.Since(m.lastSuccessfulConnect).Seconds(),
	}
}
