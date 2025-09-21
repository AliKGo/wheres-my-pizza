package server

import (
	"context"
	"net/http"
	"strconv"
	httpHandle "wheres-my-pizza/internal/adapter/http"
	"wheres-my-pizza/pkg/flags"
	"wheres-my-pizza/pkg/logger"
)

type API struct {
	mux     *http.ServeMux
	log     logger.Logger
	handler *httpHandle.OrderHandle
}

func NewRouter(logger logger.Logger, handle *httpHandle.OrderHandle) *API {
	mux := http.NewServeMux()

	return &API{
		mux:     mux,
		log:     logger,
		handler: handle,
	}
}

func (api *API) Run(ctx context.Context) {
	api.mux.Handle("/orders", api.Middleware(api.handler.CreateNewOrder()))

	if err := http.ListenAndServe(":"+strconv.Itoa(*flags.Port), api.mux); err != nil {
		api.log.Error(ctx, "error running http server", "error", err)
	}
}
