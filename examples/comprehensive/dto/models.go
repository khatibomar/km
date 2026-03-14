package dto

import "time"

type PagedResponseDTO[T any] struct {
	TotalCount int
	Data       []T
	NextPage   *string
}

type OrderDTO struct {
	ID        string
	Customer  *CustomerDTO
	Items     []OrderItemDTO
	Tags      map[string]string
	Status    string // Cast alias to primitive string
	CreatedAt time.Time
}

type CustomerDTO struct {
	ID        int
	FirstName string
	LastName  string
	Address   AddressDTO
}

type AddressDTO struct {
	Street  string
	City    string
	ZipCode string
}

type OrderItemDTO struct {
	ProductID string
	Price     float64
	Quantity  int
}
