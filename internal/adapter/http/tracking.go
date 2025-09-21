package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"wheres-my-pizza/internal/adapter/postgresql/tracking_repository"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/flags"
	"wheres-my-pizza/pkg/logger"
)

// TrackingHandler handles HTTP requests for tracking information
type TrackingHandler struct {
	repo *tracking_repository.TrackingRepository
	log  logger.Logger
	ctx  context.Context
}

// NewTrackingHandler creates a new tracking handler
func NewTrackingHandler(ctx context.Context, repo *tracking_repository.TrackingRepository) *TrackingHandler {
	return &TrackingHandler{
		repo: repo,
		log:  logger.InitLogger("tracking_handler", logger.LevelDebug),
		ctx:  ctx,
	}
}

// GetOrderStatus returns the current status of an order
func (h *TrackingHandler) GetOrderStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.log.Debug(r.Context(), types.ActionRequestReceived, "processing order status request",
			"path", r.URL.Path,
		)

		// Extract order_number from URL
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 3 {
			http.Error(w, "Invalid URL format", http.StatusBadRequest)
			return
		}

		orderNumber := pathParts[2]
		// Remove "/status" suffix if present
		orderNumber = strings.TrimSuffix(orderNumber, "/status")

		// Get order status from repository
		status, err := h.repo.GetOrderStatus(r.Context(), orderNumber)
		if err != nil {
			if strings.Contains(err.Error(), "order not found") {
				http.Error(w, "Order not found", http.StatusNotFound)
				return
			}

			h.log.Error(r.Context(), types.ActionDBQueryFailed, "failed to get order status", err,
				"order_number", orderNumber,
			)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			h.log.Error(r.Context(), types.ActionResponseFailed, "failed to encode response", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// GetOrderHistory returns the history of order status changes
func (h *TrackingHandler) GetOrderHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.log.Debug(r.Context(), types.ActionRequestReceived, "processing order history request",
			"path", r.URL.Path,
		)

		// Extract order_number from URL
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 3 {
			http.Error(w, "Invalid URL format", http.StatusBadRequest)
			return
		}

		orderNumber := pathParts[2]
		// Remove "/history" suffix if present
		orderNumber = strings.TrimSuffix(orderNumber, "/history")

		// Get order history from repository
		history, err := h.repo.GetOrderHistory(r.Context(), orderNumber)
		if err != nil {
			if strings.Contains(err.Error(), "order not found") {
				http.Error(w, "Order not found", http.StatusNotFound)
				return
			}

			h.log.Error(r.Context(), types.ActionDBQueryFailed, "failed to get order history", err,
				"order_number", orderNumber,
			)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(history); err != nil {
			h.log.Error(r.Context(), types.ActionResponseFailed, "failed to encode response", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// GetWorkersStatus returns the status of all kitchen workers
func (h *TrackingHandler) GetWorkersStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.log.Debug(r.Context(), types.ActionRequestReceived, "processing workers status request",
			"path", r.URL.Path,
		)

		// Get worker status from repository
		// Use 2 * heartbeat-interval as threshold for offline status detection
		heartbeatInterval := time.Duration(*flags.HeartbeatInterval) * time.Second
		workers, err := h.repo.GetWorkersStatus(r.Context(), 2*heartbeatInterval)
		if err != nil {
			h.log.Error(r.Context(), types.ActionDBQueryFailed, "failed to get workers status", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(workers); err != nil {
			h.log.Error(r.Context(), types.ActionResponseFailed, "failed to encode response", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}
