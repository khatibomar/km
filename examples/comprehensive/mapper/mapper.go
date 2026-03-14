package mapper

import (
	"github.com/khatibomar/km/examples/comprehensive/domain"
	"github.com/khatibomar/km/examples/comprehensive/dto"
)

//go:generate go run ../../../main.go ../../../generator.go -type OrderMapper

type OrderMapper interface {
	MapOrderList(in domain.PagedResponse[domain.Order]) dto.PagedResponseDTO[dto.OrderDTO]
	MapOrder(in domain.Order) dto.OrderDTO
}
