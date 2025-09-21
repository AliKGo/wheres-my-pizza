package http

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/internal/core/port"
	"wheres-my-pizza/pkg/flags"
)

type OrderHandle struct {
	svc port.OrderService
	ctx context.Context
}

func NewOrderHandle(context context.Context, svc port.OrderService) *OrderHandle {
	return &OrderHandle{
		svc: svc,
		ctx: context,
	}
}

func (h *OrderHandle) CreateNewOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, models.ErrorValidationFailed.Error(), http.StatusInternalServerError)
			return
		}

		if ex := h.svc.CheckConcurrent(h.ctx, *flags.MaxConcurrent); !ex {
			http.Error(w, "we have received the maximum number of orders to process", http.StatusInternalServerError)
		}

		var newOrder models.CreateOrder
		if err = json.Unmarshal(reqBody, &newOrder); err != nil {
			http.Error(w, models.ErrorValidationFailed.Error(), http.StatusBadRequest)
			return
		}

		resp, err := h.svc.CreatNewOrder(h.ctx, newOrder)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrorDbTransactionFailed):
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			case errors.Is(err, models.ErrorRabbitmqPublishFailed):
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			case errors.Is(err, models.ErrorValidationFailed):
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
