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

func TestGetImports(t *testing.T) {
	run := func(src string, expected []string) {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "main.go", src, 0)
		assert.Equal(t, nil, err)
		assert.Equal(t, expected, getImports(node))
	}

	run(`package p
	import "tt"
	import "/"`, []string{"tt", "/"})
}

func TestGetFields(t *testing.T) {
	run := func(src, typeName, expectedPkg string, expectedFields []Field) {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "main.go", src, 0)
		assert.Equal(t, nil, err)

		fields, err := getFields(node, typeName)
		assert.NoError(t, err)
		pkgName := getPackage(node)

		assert.Equal(t, expectedPkg, pkgName)

		for i, f := range fields {
			assert.Equal(t, expectedFields[i].Name, f.Name)
			assert.Equal(t, expectedFields[i].Type, f.Type)
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
			{"a", "int"},
			{"B", "string"},
		})

		run(src, "K", "p", []Field{
			{"a", "int"},
		})
	})
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

		expectedOutput := `func (dest K) FromP(src P) K {
			dest.a = src.a
			dest.B = src.B
			return dest
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

		expectedOutput := `func (dest K) FromP(src P) K {
			dest.a = src.a
			dest.B = src.B
			return dest
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

		expectedOutput := `func (dest K) FromP(src p.P) K {
			dest.B = src.B
			return dest
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

		expectedOutput := `func (dest K) FromP(src P) K {
			dest.a = src.a
			return dest
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

		expectedOutput := `func (dest K) FromP(src P) K {
			dest.a = src.a
			dest.C = src.B
			return dest
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

	t.Run("complex field in dest refereing src", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
				Meta MetaData
			}

			type MetaData struct{}
		`

		dstCode := `
			package l

			import "/bli"

			type L struct {
				a int
				B string
				Meta p.MetaData
			}
		`

		expectedOutput := `func (dest L) FromP(src p.P) L {
			dest.B = src.B
			dest.Meta = src.Meta
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/bli/p.go",
			destPath:              "/bla/l.go",
			srcName:               "P",
			destName:              "L",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("complex field in dest but not refereing src", func(t *testing.T) {
		srcCode := `
			package p

			type P struct {
				a int
				B string
				Meta MetaData
			}

			type MetaData struct{}
		`

		dstCode := `
			package l

			import p "/x"

			type L struct {
				a int
				B string
				Meta p.MetaData
			}
		`

		expectedOutput := `func (dest L) FromP(src p.P) L {
			dest.B = src.B
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/bli/p.go",
			destPath:              "/bla/l.go",
			srcName:               "P",
			destName:              "L",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("complex field in src refereing dest", func(t *testing.T) {
		srcCode := `
			package p

			import "/bla"

			type P struct {
				a int
				B string
				Meta l.MetaData
			}
		`

		dstCode := `
			package l

			type L struct {
				a int
				B string
				Meta MetaData
			}

			type MetaData struct{}
		`

		expectedOutput := `func (dest L) FromP(src p.P) L {
			dest.B = src.B
			dest.Meta = src.Meta
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/bli/file.go",
			destPath:              "/bla/file.go",
			srcName:               "P",
			destName:              "L",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("complex field in src but not refereing dest", func(t *testing.T) {
		srcCode := `
			package p

			import "/blo"

			type P struct {
				a int
				B string
				Meta l.MetaData
			}
		`

		dstCode := `
			package l

			type L struct {
				a int
				B string
				Meta MetaData
			}

			type MetaData struct{}
		`

		expectedOutput := `func (dest L) FromP(src p.P) L {
			dest.B = src.B
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/bli/file.go",
			destPath:              "/bla/file.go",
			srcName:               "P",
			destName:              "L",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
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

func TestProcess(t *testing.T) {
	var groupedWork []work

	generateWorkFromSrcs := func(srcCode, srcTypeName, dstCode, dstTypeName string) work {
		srcFset := token.NewFileSet()
		srcNode, err := parser.ParseFile(srcFset, "main.go", srcCode, 0)
		assert.NoError(t, err)

		destFset := token.NewFileSet()
		dstNode, err := parser.ParseFile(destFset, "main.go", dstCode, 0)
		assert.NoError(t, err)

		return work{
			Source: SourceData{
				node: srcNode,
				name: srcTypeName,
			},
			Destination: DestinationData{
				node: dstNode,
				name: dstTypeName,
			},
		}
	}

	t.Run("No work", func(t *testing.T) {
		g := Generator{}
		_, err := g.Process(groupedWork)
		assert.Equal(t, errNoWork, err)
	})

	t.Run("valid grouped work", func(t *testing.T) {
		code1 := `
			package p

			type S struct {
				a int
				B int
				C int
			}

			type K struct {
				a int
				B int
			}
		`
		w := generateWorkFromSrcs(code1, "K", code1, "S")
		groupedWork = append(groupedWork, w)

		code2 := `
			package p

			type T struct {
				C int
			}
		`

		expectedOutput := `package p
			func (dest S) FromK(src K) S {
				dest.a = src.a
				dest.B = src.B
				return dest
			}

			func (dest S) FromT(src T) S {
				dest.C = src.C
				return dest
			}`

		formattedExpectedOutput, err := format.Source([]byte(expectedOutput))
		assert.NoError(t, err)

		w = generateWorkFromSrcs(code2, "T", code1, "S")
		groupedWork = append(groupedWork, w)

		g := Generator{}
		f, err := g.Process(groupedWork)
		assert.NoError(t, err)
		assert.Equal(t, string(formattedExpectedOutput), string(f.Buf))
	})
}

func TestStyles(t *testing.T) {
	srcCode := `
		package p

		type S struct{
			a int
		}

		type D struct{
			a int
		}
	`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "main.go", srcCode, 0)
	assert.NoError(t, err)

	srcData := SourceData{
		node: node,
		name: "S",
	}

	dstData := DestinationData{
		node: node,
		name: "D",
	}

	t.Run("Value style", func(t *testing.T) {
		g := Generator{
			style: "value",
		}

		expectedOutput, err := format.Source(
			[]byte(`func (dest D) FromS(src S) D {
				dest.a = src.a
				return dest
			}`),
		)
		assert.NoError(t, err)

		err = g.generate(srcData, dstData)
		assert.NoError(t, err)

		assert.Equal(t, string(expectedOutput), string(g.format()))
	})

	t.Run("Pointer style", func(t *testing.T) {
		g := Generator{
			style: "pointer",
		}

		expectedOutput, err := format.Source(
			[]byte(`func (dest *D) FromS(src S) {
				dest.a = src.a
			}`),
		)
		assert.NoError(t, err)

		err = g.generate(srcData, dstData)
		assert.NoError(t, err)

		assert.Equal(t, string(expectedOutput), string(g.format()))
	})

	t.Run("Standalone style", func(t *testing.T) {
		g := Generator{
			style: "standalone",
		}

		expectedOutput, err := format.Source(
			[]byte(`func DFromS(dest D, src S) D {
				dest.a = src.a
				return dest
			}`),
		)
		assert.NoError(t, err)

		err = g.generate(srcData, dstData)
		assert.NoError(t, err)

		assert.Equal(t, string(expectedOutput), string(g.format()))
	})
}
