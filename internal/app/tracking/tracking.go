// internal/app/tracking/tracking.go
package tracking

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	handle "wheres-my-pizza/internal/adapter/http"
	"wheres-my-pizza/internal/adapter/postgresql/tracking_repository"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/flags"
	"wheres-my-pizza/pkg/logger"
)

// TrackingApp represents the tracking service application
type TrackingApp struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  logger.Logger
	repo    *tracking_repository.TrackingRepository
	handler *handle.TrackingHandler
	port    int
}

// NewTrackingApp creates a new tracking service application
func NewTrackingApp() *TrackingApp {
	cfg, err := config.ParseYAML()
	if err != nil {
		config.PrintYAMLHelp()
		slog.Error("failed to configure application", "error", err)
		os.Exit(1)
	}

	logger := logger.InitLogger("Tracking Service", logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())

	// Connect to database
	repo, err := tracking_repository.NewTrackingRepository(ctx, cfg)
	if err != nil {
		cancel()
		logger.Error(ctx, types.ActionDBConnectFailed, "failed to connect to database", err)
		os.Exit(1)
	}

	// Create HTTP handler
	handler := handle.NewTrackingHandler(ctx, repo)

	return &TrackingApp{
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		repo:    repo,
		handler: handler,
		port:    *flags.Port,
	}
}

// Start begins the tracking service operation
func (app *TrackingApp) Start() {
	// Create HTTP router
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/orders/", app.routeOrdersRequests)
	mux.HandleFunc("/workers/status", app.handler.GetWorkersStatus())

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + strconv.Itoa(app.port),
		Handler: mux,
	}

	app.logger.Info(app.ctx, types.ActionServiceStarted, "tracking service started",
		"port", app.port,
	)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Error(app.ctx, types.ActionServiceFailed, "server error", err,
				"port", app.port,
			)
			app.cancel()
		}
	}()

	// Wait for signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	app.logger.Info(app.ctx, types.ActionGracefulShutdown, "service is shutting down")
	app.cancel()

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		app.logger.Error(app.ctx, types.ActionGracefulShutdown, "error during server shutdown", err)
	}

	// Close database connection
	if app.repo != nil {
		app.repo.Close()
	}
}

// routeOrdersRequests handles routing for /orders/ endpoints
func (app *TrackingApp) routeOrdersRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Route to appropriate handler based on URL path
	if strings.HasSuffix(path, "/status") {
		app.handler.GetOrderStatus()(w, r)
		return
	}

	if strings.HasSuffix(path, "/history") {
		app.handler.GetOrderHistory()(w, r)
		return
	}

	// If no matching path pattern, return 404
	http.NotFound(w, r)
}
