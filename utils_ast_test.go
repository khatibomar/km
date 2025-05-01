package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestExtractTypeFromExpression(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		typeName string
		want     string
	}{
		{
			name:     "basic type",
			code:     "type T string",
			typeName: "T",
			want:     "string",
		},
		{
			name:     "pointer type",
			code:     "type T *int",
			typeName: "T",
			want:     "*int",
		},
		{
			name:     "slice type",
			code:     "type T []string",
			typeName: "T",
			want:     "[]string",
		},
		{
			name:     "map type",
			code:     "type T map[string]int",
			typeName: "T",
			want:     "map[string]int",
		},
		{
			name:     "struct type",
			code:     "type T struct{}",
			typeName: "T",
			want:     "struct{}",
		},
		{
			name:     "interface type",
			code:     "type T interface{}",
			typeName: "T",
			want:     "interface{}",
		},
		{
			name:     "interface type",
			code:     "type T any",
			typeName: "T",
			want:     "any",
		},
		{
			name:     "channel type",
			code:     "type T chan string",
			typeName: "T",
			want:     "chan string",
		},
		{
			name:     "send channel type",
			code:     "type T chan<- string",
			typeName: "T",
			want:     "chan<- string",
		},
		{
			name:     "receive channel type",
			code:     "type T <-chan string",
			typeName: "T",
			want:     "<-chan string",
		},
		{
			name:     "selector type",
			code:     "type T fmt.Stringer",
			typeName: "T",
			want:     "fmt.Stringer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", "package p\n"+tt.code, 0)
			if err != nil {
				t.Fatal(err)
			}

			var typeExpr ast.Expr
			ast.Inspect(f, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == tt.typeName {
					typeExpr = ts.Type
					return false
				}
				return true
			})

			if got := extractTypeFromExpression(typeExpr); got != tt.want {
				t.Errorf("extractTypeFromExpression() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFields(t *testing.T) {
	run := func(src, typeName, expectedPkg string, expectedFields []Field) {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "main.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}

		fields, err := getFields(node, typeName)
		if err != nil {
			t.Fatal(err)
		}

		pkgName := getPackage(node)

		if pkgName != expectedPkg {
			t.Errorf("Expected package %s but got %s", expectedPkg, pkgName)
		}

		if len(fields) != len(expectedFields) {
			t.Errorf("Expected %d fields but got %d", len(expectedFields), len(fields))
			return
		}

		for i, f := range fields {
			if f.Name != expectedFields[i].Name {
				t.Errorf("Expected field name %s but got %s", expectedFields[i].Name, f.Name)
			}
			if f.Type != expectedFields[i].Type {
				t.Errorf("Expected field type %s but got %s", expectedFields[i].Type, f.Type)
			}
			if (f.Children == nil) != (expectedFields[i].Children == nil) {
				t.Errorf("Expected children nil status %v but got %v", expectedFields[i].Children == nil, f.Children == nil)
			}
			if f.Children != nil && expectedFields[i].Children != nil {
				if len(*f.Children) != len(*expectedFields[i].Children) {
					t.Errorf("Expected %d children but got %d", len(*expectedFields[i].Children), len(*f.Children))
				} else {
					for j, child := range *f.Children {
						expectedChild := (*expectedFields[i].Children)[j]
						if child.Name != expectedChild.Name {
							t.Errorf("Expected child name %s but got %s", expectedChild.Name, child.Name)
						}
						if child.Type != expectedChild.Type {
							t.Errorf("Expected child type %s but got %s", expectedChild.Type, child.Type)
						}
					}
				}
			}
		}
	}

	t.Run("simple struct", func(t *testing.T) {
		src := `
		package p

		type P struct {
			a	int
			B	string
		}

		type K struct {
			a int
		}
		`
		run(src, "P", "p", []Field{
			{"a", "int", nil},
			{"B", "string", nil},
		})

		run(src, "K", "p", []Field{
			{"a", "int", nil},
		})
	})

	t.Run("nested structs", func(t *testing.T) {
		src := `
		package p

		type Address struct {
			Street string
			City   string
		}

		type Person struct {
			Name    string
			Address Address
		}
		`
		children := []Field{
			{"Street", "string", nil},
			{"City", "string", nil},
		}
		run(src, "Person", "p", []Field{
			{"Name", "string", nil},
			{"Address", "Address", &children},
		})
	})

	t.Run("map types", func(t *testing.T) {
		src := `
		package p

		type StringIntMap map[string]int
		type ComplexMap map[*string][]int
		type NestedMap map[string]map[int]bool
		`

		run(src, "StringIntMap", "p", []Field{
			{"key", "string", nil},
			{"value", "int", nil},
		})

		run(src, "ComplexMap", "p", []Field{
			{"key", "*string", nil},
			{"value", "[]int", nil},
		})

		run(src, "NestedMap", "p", []Field{
			{"key", "string", nil},
			{"value", "map[int]bool", nil},
		})
	})

	t.Run("type not found", func(t *testing.T) {
		src := `
		package p
		type X struct{}
		`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "main.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}

		_, err = getFields(node, "NonExistentType")
		if err == nil {
			t.Error("Expected error for non-existent type, got nil")
		}
	})
}
