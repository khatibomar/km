package mapper

//go:generate go run ../../../main.go ../../../generator.go -mapping "github.com/khatibomar/km/examples/generics/domain.Result[Product]=github.com/khatibomar/km/examples/generics/dto.ResultDTO[ProductDTO]" -output ./generics_mapper_gen.go -package mapper -package-path github.com/khatibomar/km/examples/generics/mapper

