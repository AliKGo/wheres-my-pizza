package restaurant

import (
	"wheres-my-pizza/internal/app/kitchen"
	"wheres-my-pizza/internal/app/notification"
	"wheres-my-pizza/internal/app/order"
	"wheres-my-pizza/internal/app/tracking"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/pkg/flags"
)

// Run initializes and starts the appropriate service based on the mode flag
func Run() {
	flags.ParseFlag()

	switch *flags.Mode {
	case types.ModeOrderService:
		app := order.NewOrderApp()
		app.Start()

	case types.ModeKitchenWorker:
		app := kitchen.NewKitchenApp()
		app.Start()

	case types.ModeTrackingService:
		app := tracking.NewTrackingApp()
		app.Start()

	case types.ModeNotificationSubscriber:
		app := notification.NewNotificationApp()
		app.Start()
	}
}
