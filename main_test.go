package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type testCase struct {
	name          string
	domainModels  string
	dtoModels     string
	mapperCode    string
	interfaceName string
}

func TestMappers(t *testing.T) {
	tests := []testCase{
		{
			name: "Basic Struct Mapping",
			domainModels: `package domain
type User struct { Name string; Age int }`,
			dtoModels: `package dto
type UserDTO struct { Name string; Age int }`,
			mapperCode: `package mapper
import ("testpkg/domain"; "testpkg/dto")
type Mapper interface { MapUser(in domain.User) dto.UserDTO }`,
			interfaceName: "Mapper",
		},
		{
			name: "Pointer To Value",
			domainModels: `package domain
type Order struct { Customer *Customer }
type Customer struct { Name string }`,
			dtoModels: `package dto
type OrderDTO struct { Customer CustomerDTO }
type CustomerDTO struct { Name string }`,
			mapperCode: `package mapper
import ("testpkg/domain"; "testpkg/dto")
type Mapper interface { MapOrder(in domain.Order) dto.OrderDTO }`,
			interfaceName: "Mapper",
		},
		{
			name: "Method Reuse",
			domainModels: `package domain
type Order struct { Customer *Customer }
type Customer struct { Name string }`,
			dtoModels: `package dto
type OrderDTO struct { Customer CustomerDTO }
type CustomerDTO struct { Name string }`,
			mapperCode: `package mapper
import ("testpkg/domain"; "testpkg/dto")
type Mapper interface {
	MapOrder(in domain.Order) dto.OrderDTO
	MapCustomerEx(in *domain.Customer) dto.CustomerDTO
}`,
			interfaceName: "Mapper",
		},
		{
			name: "Map with pointers and slices",
			domainModels: `package domain
type Order struct { Items []OrderItem; Tags map[string]Customer }
type OrderItem struct { ProductID string }
type Customer struct { Name string }`,
			dtoModels: `package dto
type OrderDTO struct { Items *[]OrderItemDTO; Tags map[string]*CustomerDTO }
type OrderItemDTO struct { ProductID string }
type CustomerDTO struct { Name string }`,
			mapperCode: `package mapper
import ("testpkg/domain"; "testpkg/dto")
type Mapper interface { MapOrder(in domain.Order) dto.OrderDTO }`,
			interfaceName: "Mapper",
		},
		{
			name: "Time.Time test and embedded struct",
			domainModels: `package domain
import "time"
type Base struct { ID string; CreatedAt time.Time }
type Order struct { Base }`,
			dtoModels: `package dto
import "time"
type OrderDTO struct { ID string; CreatedAt time.Time }`,
			mapperCode: `package mapper
import ("testpkg/domain"; "testpkg/dto")
type Mapper interface { MapOrder(in domain.Order) dto.OrderDTO }`,
			interfaceName: "Mapper",
		},
		{
			name: "Bytes to String casting natively",
			domainModels: `package domain
type Data struct { Payload []byte; Runes []rune }`,
			dtoModels: `package dto
type DataDTO struct { Payload string; Runes string }`,
			mapperCode: `package mapper
import ("testpkg/domain"; "testpkg/dto")
type Mapper interface { MapData(in domain.Data) dto.DataDTO }`,
			interfaceName: "Mapper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module testpkg\n\ngo 1.25\n"), 0644)
			os.WriteFile(filepath.Join(domainDir, "models.go"), []byte(tt.domainModels), 0644)
			os.WriteFile(filepath.Join(dtoDir, "models.go"), []byte(tt.dtoModels), 0644)
			os.WriteFile(filepath.Join(mapperDir, "mapper.go"), []byte(tt.mapperCode), 0644)

			config := GeneratorConfig{
				WorkingDir:    mapperDir,
				InterfaceName: tt.interfaceName,
				OutputFile:    filepath.Join(mapperDir, "km_gen.go"),
			}

			if err := Generate(config); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Try compiling the result
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = tempDir
			if output, err := cmd.CombinedOutput(); err != nil {
				outBytes, _ := os.ReadFile(config.OutputFile)
				t.Fatalf("Compile failed: %v\nOutput: %s\nGenerated code:\n%s", err, output, string(outBytes))
			}
		})
	}
}
