package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndToEndMappers(t *testing.T) {
	// Set up a temporary directory
	tempDir, err := os.MkdirTemp("", "km_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	domainDir := filepath.Join(tempDir, "domain")
	dtoDir := filepath.Join(tempDir, "dto")
	mapperDir := filepath.Join(tempDir, "mapper")

	os.MkdirAll(domainDir, 0755)
	os.MkdirAll(dtoDir, 0755)
	os.MkdirAll(mapperDir, 0755)

	os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module testpkg\n\ngo 1.21\n"), 0644)

	// Write complex domain models
	os.WriteFile(filepath.Join(domainDir, "models.go"), []byte(`
package domain

type PagedResponse[T any] struct {
	TotalCount int
	Data       []T
	NextPage   *string
}

type Order struct {
	ID            string
	Customer      *Customer
	Items         []OrderItem
	Tags          map[string]string
	Status        OrderStatus
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
`), 0644)

	// Write complex DTO models
	os.WriteFile(filepath.Join(dtoDir, "models.go"), []byte(`
package dto

type PagedResponseDTO[T any] struct {
	TotalCount int
	Data       []T
	NextPage   *string
}

type OrderDTO struct {
	ID            string
	Customer      *CustomerDTO
	Items         []OrderItemDTO
	Tags          map[string]string
	Status        string // Cast alias to primitive string
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
`), 0644)

	// Write mapper containing multiple advanced tests
	mapperCode := `package mapper
import (
"testpkg/domain"
"testpkg/dto"
)

type ProductionMapper interface {
    // Basic Mapping
	MapAddress(in domain.Address) dto.AddressDTO
    
    // Arrays and underlying type casts (OrderStatus -> string)
	MapOrderItems(in []domain.OrderItem) []dto.OrderItemDTO

    // Nested Pointers
	MapCustomer(in *domain.Customer) *dto.CustomerDTO

    // Deeply Nested Map + Generic Instantiation
	MapOrderList(in domain.PagedResponse[domain.Order]) dto.PagedResponseDTO[dto.OrderDTO]
}
`
	os.WriteFile(filepath.Join(mapperDir, "mapper.go"), []byte(mapperCode), 0644)

	// Run generation
	config := GeneratorConfig{
		WorkingDir:    mapperDir,
		InterfaceName: "ProductionMapper",
		OutputFile:    filepath.Join(mapperDir, "km_productionmapper_gen.go"),
	}

	if err := Generate(config); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify generated file
	genCode, err := os.ReadFile(config.OutputFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	// Some basic assertions
	if !strings.Contains(string(genCode), "type ProductionMapperImpl struct{}") {
		t.Errorf("Generated code missing ProductionMapperImpl")
	}

	// Try compiling the result
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tempDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Compile failed: %v\nOutput: %s", err, output)
	}
}
