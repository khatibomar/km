package domain

import "time"

type PagedResponse[T any] struct {
	TotalCount int
	Data       []T
	NextPage   *string
}

type Order struct {
	ID        string
	Customer  *Customer
	Items     []OrderItem
	Tags      map[string]string
	Status    OrderStatus
	CreatedAt time.Time
}

type Customer struct {
	ID        int
	FirstName string
	LastName  string
	Address   Address
}

type Address struct {
	Street  string
	City    string
	ZipCode string
}

type OrderItem struct {
	ProductID string
	Price     float64
	Quantity  int
}

type OrderStatus string
