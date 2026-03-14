package domain

type Result[T any] struct {
	Data       T
	TotalCount int
	Errors     []string
}

type Product struct {
	SKU   string
	Price float64
}
