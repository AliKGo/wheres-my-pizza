package rabbitmq

import (
	"encoding/json"
	"github.com/rabbitmq/amqp091-go"
)

func ExtractOrderNumber(delivery amqp091.Delivery) (string, error) {
	var message struct {
		OrderNumber string `json:"order_number"`
	}

	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		return "", err
	}

	return message.OrderNumber, nil
}
