package dto

type ResultDTO[T any] struct {
	Data       T
	TotalCount int64
	Errors     []string
}

type ProductDTO struct {
	SKU   string
	Price float64
}
