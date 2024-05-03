package main

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
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

	fields, err := getFields(node, "P")
	assert.NoError(t, err)
	pkgName := getPackage(node)

	assert.Equal(t, "p", pkgName, "")
	assert.Len(t, fields, 2)

	assert.Equal(t, fields[0].Name, "a")
	assert.Equal(t, fields[0].Type, "int")

	assert.Equal(t, fields[1].Name, "B")
	assert.Equal(t, fields[1].Type, "string")
}

func TestGenerate(t *testing.T) {
	type testInput struct {
		srcCode               string
		destCode              string
		sourcePath            string
		destPath              string
		srcName               string
		destName              string
		expectedOutput        string
		expectedGenerateError error
		ignoredFields         []string
		fieldsMap             map[string]string
	}

	runTest := func(t *testing.T, input testInput) {
		srcFset := token.NewFileSet()
		srcNode, err := parser.ParseFile(srcFset, "main.go", input.srcCode, 0)
		assert.NoError(t, err)

		destFset := token.NewFileSet()
		dstNode, err := parser.ParseFile(destFset, "main.go", input.destCode, 0)
		assert.NoError(t, err)

		g := Generator{}

		source := SourceData{
			node: srcNode,
			path: input.sourcePath,
			name: input.srcName,
		}

		ignoredMap := make(map[string]struct{})

		for _, i := range input.ignoredFields {
			ignoredMap[i] = struct{}{}
		}

		destination := DestinationData{
			node:       dstNode,
			path:       input.destPath,
			name:       input.destName,
			ignoredMap: ignoredMap,
			fieldsMap:  input.fieldsMap,
		}

		err = g.generate(source, destination)
		assert.Equal(t, input.expectedGenerateError, err)

		expectedFormatted, err := format.Source([]byte(input.expectedOutput))
		assert.NoError(t, err)

		assert.Equal(t, string(expectedFormatted), string(g.format()))
	}

	t.Run("both structs are in same package different files", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
			}
		`
		destCode := `
			package p

			type K struct {
				a int
				B string
			}
		`

		expectedOutput := `func (dest *P) FromK(src K) {
			dest.a = src.a
			dest.B = src.B
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              destCode,
			sourcePath:            "/p.go",
			destPath:              "/k.go",
			srcName:               "P",
			destName:              "K",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("both structs are in same package same file", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
			}

			type K struct {
				a int
				B string
			}
		`

		expectedOutput := `func (dest *P) FromK(src K) {
			dest.a = src.a
			dest.B = src.B
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              srcCode,
			sourcePath:            "/p.go",
			destPath:              "/p.go",
			srcName:               "P",
			destName:              "K",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("both structs are in different packages", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
			}
		`
		destCode := `
			package k

			type K struct {
				a int
				B string
			}
		`

		expectedOutput := `func (dest *P) FromK(src k.K) {
			dest.B = src.B
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              destCode,
			sourcePath:            "/p.go",
			destPath:              "/test/k.go",
			srcName:               "P",
			destName:              "K",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("struct in configuration doesn't exist", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
			}
		`
		destCode := `
			package k

			type S struct {
				a int
				B string
			}
		`

		expectedOutput := ``

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              destCode,
			sourcePath:            "/p.go",
			destPath:              "/test/k.go",
			srcName:               "P",
			destName:              "K",
			expectedOutput:        expectedOutput,
			expectedGenerateError: fmt.Errorf("type(K): %w", errTypeNotFound),
		})
	})

	t.Run("ignore fields", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
			}

			type K struct {
				a int
				B string
			}
		`

		expectedOutput := `func (dest *P) FromK(src K) {
			dest.a = src.a
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              srcCode,
			sourcePath:            "/p.go",
			destPath:              "/p.go",
			srcName:               "P",
			destName:              "K",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
			ignoredFields:         []string{"B"},
		})
	})

	t.Run("mapping fields", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
			}

			type K struct {
				a int
				C string
			}
		`

		expectedOutput := `func (dest *P) FromK(src K) {
			dest.a = src.a
			dest.C = src.B
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              srcCode,
			sourcePath:            "/p.go",
			destPath:              "/p.go",
			srcName:               "P",
			destName:              "K",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
			fieldsMap: map[string]string{
				"C": "B",
			},
		})
	})
}

func TestGroupMappings(t *testing.T) {
	var mappings []Mapping
	mappings = append(mappings, Mapping{
		Destination: Destination{
			Path: "/dir/d1/file.go",
		},
	})
	mappings = append(mappings, Mapping{
		Destination: Destination{
			Path: "/dir/d1/file2.go",
		},
	})
	mappings = append(mappings, Mapping{
		Destination: Destination{
			Path: "/dir/d2/file1.go",
		},
	})
	res := groupMappings(mappings)
	assert.Len(t, res, 2)

	dirD1 := fmt.Sprintf("%c%s%c%s", os.PathSeparator, "dir", os.PathSeparator, "d1")
	dirD2 := fmt.Sprintf("%c%s%c%s", os.PathSeparator, "dir", os.PathSeparator, "d2")

	assert.Equal(t, mappings[:2], res[dirD1])
	assert.Equal(t, mappings[2:], res[dirD2])
}
