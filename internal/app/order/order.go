package order

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"wheres-my-pizza/internal/adapter/http"
	"wheres-my-pizza/internal/adapter/postgresql/order_repository"
	"wheres-my-pizza/internal/adapter/server"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/internal/core/serivce/order"
	"wheres-my-pizza/pkg/config"
	"wheres-my-pizza/pkg/logger"
)

type OrderApp struct {
	api    *server.API
	ctx    context.Context
	cancel context.CancelFunc
	logger logger.Logger
}

func NewOrderApp() *OrderApp {
	cfg, err := config.ParseYAML()
	if err != nil {
		config.PrintYAMLHelp()
		slog.Error("failed to configure application", err)
		os.Exit(1)
	}

	logger := logger.InitLogger("Order Service", logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	repo, err := order_repository.NewOrderRepository(ctx, cfg)
	if err != nil {
		cancel()
		config.PrintYAMLHelp()
		slog.Error("failed to configure application", err)
		os.Exit(1)
	}

	svc := order.NewOrderService(repo)
	handle := http.NewOrderHandle(ctx, svc)

	api := server.NewRouter(logger, handle)

	return &OrderApp{
		api,
		ctx,
		cancel,
		logger,
	}
}

func (app *OrderApp) Start() {
	app.api.Run(app.ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	app.cancel()
	app.logger.Info(app.ctx, types.ActionGracefulShutdown, "service is shutting down")
}
