# KM (Khatibomar Mapper)

A highly robust **Interface-Driven Compile-Time Code Generator** for deep, nested, and generic Go struct mapping.

KM evaluates your interfaces at compile time using Go's `go/types` analysis and statically generates recursive mapper functions. No `reflect` package at runtime, zero overhead, and fully type-safe!

## Features
- **Zero Runtime Overhead:** No reflection! Generates raw Go assignment loops.
- **Deeply Nested Support:** Recursively travels through Structs, Slices, Maps, and Arrays.
- **Interface Driven (DSL):** Simply define what goes in and what comes out in a Go `interface` and let `km` generate the exact standalone functions matching your signatures.
- **Generics Supported:** Parses type parameters dynamically.
- **Type Casting & Alignment:** Converts underlying types down the hierarchy flawlessly.

## Usage

Simply define an interface and drop a `//go:generate` tag on top. The interface acts entirely as a structural schema!

```go
package mapper

import (
"github.com/my/project/domain"
"github.com/my/project/dto"
)

//go:generate go run github.com/khatibomar/km -type MapperSchema

// MapperSchema is strictly used by the code generator to scaffold your mapping functions.
type MapperSchema interface {
    MapUser(in domain.User) dto.UserDTO
    MapProducts(in []domain.Product) []dto.ProductDTO
}
```

Run:
```bash
go generate ./...
```

It will generate a `<your_interface>_gen.go` file right alongside it containing purely standalone and performant functions matching the method names exactly!

```go
func MapUser(in domain.User) dto.UserDTO { ... }
func MapProducts(in []domain.Product) []dto.ProductDTO { ... }
```

Check out the `examples/` directory for robust showcases!
