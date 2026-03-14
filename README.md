# KM (Khatibomar Mapper)

A highly robust **Interface-Driven Compile-Time Code Generator** for deep, nested, and generic Go struct mapping.

KM evaluates your interfaces at compile time using Go's `go/types` analysis and statically generates recursive mappers. No `reflect` package at runtime, zero overhead, and fully type-safe!

## Features
- **Zero Runtime Overhead:** No reflection! Generates raw Go assignment loops.
- **Deeply Nested Support:** Recursively travels through Structs, Slices, Maps, and Arrays.
- **Interface Driven:** Simply define what goes in and what comes out in a Go `interface` and let `km` handle the rest.
- **Generics Supported:** Parses type parameters dynamically.
- **Type Casting & Alignment:** Converts underlying types down the hierarchy flawlessly.

## Usage

Simply define an interface and drop a `//go:generate` tag on top:

```go
package mapper

import (
"github.com/my/project/domain"
"github.com/my/project/dto"
)

//go:generate go run github.com/khatibomar/km -type UserMapper

type UserMapper interface {
    MapUser(in domain.User) dto.UserDTO
    MapProducts(in []domain.Product) []dto.ProductDTO
}
```

Run:
```bash
go generate ./...
```

It will generate a `<your_interface>_gen.go` file right alongside it containing the full mapping implementation: `NewUserMapper() UserMapper`.

Check out the `examples/` directory for robust showcases!
