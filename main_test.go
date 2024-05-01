package main

import (
	"go/format"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFields(t *testing.T) {
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

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "main.go", src, 0)
	assert.Equal(t, nil, err)

	fields := getFields(node, "P")
	pkgName := getPackage(node)

	assert.Equal(t, "p", pkgName, "")
	assert.Len(t, fields, 2)

	assert.Equal(t, fields[0].Name, "a")
	assert.Equal(t, fields[0].Type, "int")

	assert.Equal(t, fields[1].Name, "B")
	assert.Equal(t, fields[1].Type, "string")
}

func TestGenerate(t *testing.T) {
	srcCode := `
			package p

			type P struct {
				a	int
				B	string
			}
		`
	destCode := `
			package p

			type K struct {
				a	int
				B	string
			}
		`

	srcFset := token.NewFileSet()
	srcNode, err := parser.ParseFile(srcFset, "main.go", srcCode, 0)
	assert.Equal(t, nil, err)

	destFset := token.NewFileSet()
	dstNode, err := parser.ParseFile(destFset, "main.go", destCode, 0)
	assert.Equal(t, nil, err)

	t.Run("both structs are in same package", func(t *testing.T) {
		g := Generator{}

		source := SourceData{
			node: srcNode,
			path: "/p.go",
			name: "P",
		}

		destination := DestinationData{
			node: dstNode,
			path: "/k.go",
			name: "K",
		}

		output := `func (dest *P) FromK(src K) {
			dest.a = src.a
			dest.B = src.B
		}`

		err = g.generate(source, destination)
		assert.ErrorIs(t, err, nil)

		expected, err := format.Source([]byte(output))
		assert.ErrorIs(t, err, nil)

		assert.Equal(t, string(expected), string(g.format()))
	})

	t.Run("both structs are in different package", func(t *testing.T) {
		g := Generator{}

		source := SourceData{
			node: srcNode,
			path: "/p.go",
			name: "P",
		}

		destination := DestinationData{
			node: dstNode,
			path: "/test/k.go",
			name: "K",
		}

		output := `func (dest *P) FromK(src p.K) {
			dest.B = src.B
		}`

		err = g.generate(source, destination)
		assert.ErrorIs(t, err, nil)

		expected, err := format.Source([]byte(output))
		assert.ErrorIs(t, err, nil)

		assert.Equal(t, string(expected), string(g.format()))
	})
}
