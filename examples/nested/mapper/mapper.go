package mapper

//go:generate go run ../../../main.go ../../../generator.go -mapping github.com/khatibomar/km/examples/nested/domain.Employee=github.com/khatibomar/km/examples/nested/dto.EmployeeDTO -output ./nested_mapper_gen.go -package mapper -package-path github.com/khatibomar/km/examples/nested/mapper
