package order

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
	"wheres-my-pizza/internal/core/domain/models"
	"wheres-my-pizza/internal/core/domain/types"
	"wheres-my-pizza/internal/core/port"
	"wheres-my-pizza/pkg/logger"
)

type Service struct {
	log    logger.Logger
	db     port.OrderRepository
	rabbit port.RabbitMQ
}

func NewOrderService(db port.OrderRepository) *Service {
	log := logger.InitLogger("order", logger.LevelDebug)
	return &Service{
		db:  db,
		log: log,
	}
}

func (svc *Service) CreatNewOrder(ctx context.Context, newOrder models.CreateOrder) (models.OrderResponse, error) {
	var orderResponse models.OrderResponse
	err := validateOrder(newOrder)
	if err != nil {
		svc.log.Error(ctx, types.ActionOrderReceived, "error in validate order", err)
		return models.OrderResponse{}, models.ErrorValidationFailed
	}

	for _, item := range newOrder.OrderItems {
		orderResponse.TotalAmount += item.Price * float64(item.Quantity)
	}
	newOrder.TotalPrice = orderResponse.TotalAmount

	newOrder.Priority = priority(orderResponse.TotalAmount)

	var createAt time.Time

	orderResponse.OrderNumber, createAt, err = svc.db.CreateNewOrder(ctx, newOrder)
	if err != nil {
		svc.log.Error(ctx, types.ActionOrderReceived, "error in create order", err)
	}
	fmt.Println(createAt)

	// тут надо добавить отправку в rabbitMQ b c этим orders закончится

	return orderResponse, nil
}

func validateOrder(order models.CreateOrder) error {
	if len(order.CustomerName) < 1 || len(order.CustomerName) > 100 {
		return errors.New("customer_name must be 1-100 characters")
	}
	validName := regexp.MustCompile(`^[a-zA-Z\s'-]+$`)
	if !validName.MatchString(order.CustomerName) {
		return errors.New("customer_name contains invalid characters")
	}

	if order.OrderType != "dine_in" && order.OrderType != "takeout" && order.OrderType != "delivery" {
		return errors.New("order_type must be one of: 'dine_in', 'takeout', 'delivery'")
	}

	if len(order.OrderItems) < 1 || len(order.OrderItems) > 20 {
		return errors.New("items must contain 1-20 elements")
	}

	for i, item := range order.OrderItems {
		if len(item.Name) < 1 || len(item.Name) > 50 {
			return fmt.Errorf("items[%d].name must be 1-50 characters", i)
		}
		if item.Quantity < 1 || item.Quantity > 10 {
			return fmt.Errorf("items[%d].quantity must be between 1 and 10", i)
		}
		if item.Price < 0.01 || item.Price > 999.99 {
			return fmt.Errorf("items[%d].price must be between 0.01 and 999.99", i)
		}
	}

	switch order.OrderType {
	case "dine_in":
		if order.TableNumber == nil || *order.TableNumber < 1 || *order.TableNumber > 100 {
			return errors.New("table_number must be 1-100 for dine_in orders")
		}
		if order.DeliveryAddress != nil {
			return errors.New("delivery_address must not be present for dine_in orders")
		}
	case "delivery":
		if order.DeliveryAddress == nil || len(*order.DeliveryAddress) < 10 {
			return errors.New("delivery_address must be at least 10 characters for delivery orders")
		}
		if order.TableNumber != nil {
			return errors.New("table_number must not be present for delivery orders")
		}
	case "takeout":
		if order.TableNumber != nil {
			return errors.New("table_number must not be present for takeout orders")
		}
		if order.DeliveryAddress != nil {
			return errors.New("delivery_address must not be present for takeout orders")
		}
	}

	return nil
}

func priority(totalPrice float64) int {
	if totalPrice > 100 {
		return 10
	} else if 50 <= totalPrice && totalPrice < 100 {
		return 5
	} else {
		return 1
	}

}

// true можно false нельзя
func (svc *Service) CheckConcurrent(ctx context.Context, limit int) bool {
	count, err := svc.db.GetNumberOrdersProcessed(ctx)
	if err != nil {
		svc.log.Error(ctx, types.ActionOrderReceived, "error in check concurrent", err)
		return false
	}
	if count < limit {
		return true
	}
	return false
}
