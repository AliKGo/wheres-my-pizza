package models

type CreateOrder struct {
	CustomerName    string      `json:"customer_name"`
	OrderType       string      `json:"order_type"`
	OrderItems      []OrderItem `json:"items"`
	TableNumber     *int        `json:"table_number,omitempty"`
	DeliveryAddress *string     `json:"delivery_address,omitempty"`
	TotalPrice      float64
	Priority        int
}

type OrderItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type OrderResponse struct {
	OrderNumber string  `json:"order_number"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
}
