package main

import (
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

	pkgName, fields := getFields(node, "P")

	assert.Equal(t, "p", pkgName, "")
	assert.Len(t, fields, 2)

	assert.Equal(t, fields["a"].Name, "a")
	assert.Equal(t, fields["a"].Type, "int")

	assert.Equal(t, fields["B"].Name, "B")
	assert.Equal(t, fields["B"].Type, "string")
}
