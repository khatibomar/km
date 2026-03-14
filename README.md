# KM (Khatibomar Mapper)

A highly robust **Compile-Time Code Generator** for deep, nested, and generic Go struct mapping.

KM evaluates your struct hierarchies at compile time using Go's `go/types` analysis and strictly statically types recursive mappers. No `reflect` package at runtime, zero overhead, and fully type-safe!

## Features
- **Zero Runtime Overhead:** No reflection! Generates raw Go assignment loops.
- **Deeply Nested Support:** Recursively travels through Structs, Slices, Maps, and Arrays.
- **Generics Supported:** Parses type parameters dynamically and structures the mapper perfectly logic.
- **Type Casting & Alignment:** Converts underneath types flawlessly down the hierarchy.

## Usage

Simply drop a `//go:generate` tag in a `mapper.go` file:

```go
package mapper

//go:generate go run github.com/khatibomar/km -mapping "github.com/my/project/domain.Result[Product]=github.com/my/project/dto.ResultDTO[ProductDTO]" -output ./generated_mapper.go -package mapper -package-path github.com/my/project/mapper
```

Run:
```bash
go generate ./...
```

Check out the `examples/` directory for robust showcases!
