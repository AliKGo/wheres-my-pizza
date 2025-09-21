package restaurant

import (
	"wheres-my-pizza/internal/app/order"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/flags"
)

func Run() {
	flags.ParseFlag()
	switch *flags.Mode {
	case types.ModeOrderService:
		app := order.NewOrderApp()
		app.Start()
	}
}
