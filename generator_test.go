package main

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"testing"
)

func init() {
	version = "test"
}

func TestGetImports(t *testing.T) {
	run := func(src string, expected []string) {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "main.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}

		got := getImports(node)
		if len(got) != len(expected) {
			t.Errorf("Expected imports %v but got %v", expected, got)
		}

		for i := range got {
			if got[i] != expected[i] {
				t.Errorf("Expected import %s but got %s at index %d", expected[i], got[i], i)
			}
		}
	}

	run(`package p
	import "tt"
	import "/"`, []string{"tt", "/"})
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
		if err != nil {
			t.Fatal(err)
		}

		destFset := token.NewFileSet()
		dstNode, err := parser.ParseFile(destFset, "main.go", input.destCode, 0)
		if err != nil {
			t.Fatal(err)
		}

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
		if err != input.expectedGenerateError {
			if err == nil {
				t.Errorf("Expected error %v but got nil", input.expectedGenerateError)
			} else if input.expectedGenerateError == nil {
				t.Errorf("Expected nil error but got %v", err)
			} else if err.Error() != input.expectedGenerateError.Error() {
				t.Errorf("Expected error %v but got %v", input.expectedGenerateError, err)
			}
		}

		expectedFormatted, err := format.Source([]byte(input.expectedOutput))
		if err != nil {
			t.Fatal(err)
		}

		if string(expectedFormatted) != string(g.format()) {
			t.Errorf("Expected output:\n%s\n\nBut got:\n%s", string(expectedFormatted), string(g.format()))
		}
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
		Destinations: []Destination{
			{
				Path: filepath.Join("dir", "d1", "file.go"),
			},
			{
				Path: filepath.Join("dir", "d2", "file2.go"),
			},
		},
	})
	mappings = append(mappings, Mapping{
		Destinations: []Destination{
			{
				Path: filepath.Join("dir", "d1", "file2.go"),
			},
			{
				Path: filepath.Join("dir", "d3", "file3.go"),
			},
		},
	})
	mappings = append(mappings, Mapping{
		Destinations: []Destination{
			{
				Path: filepath.Join("dir", "d2", "file1.go"),
			},
		},
	})
	res := groupMappings(mappings)
	if len(res) != 3 {
		t.Errorf("Expected 3 groups but got %d", len(res))
	}

	dirD1 := filepath.Join("dir", "d1")
	dirD2 := filepath.Join("dir", "d2")
	dirD3 := filepath.Join("dir", "d3")

	expectedD1 := []Mapping{mappings[0], mappings[1]}
	expectedD2 := []Mapping{mappings[0], mappings[2]}
	expectedD3 := []Mapping{mappings[1]}

	if !equalMappingSlice(res[dirD1], expectedD1) {
		t.Errorf("Expected mappings %v but got %v for dir %s", expectedD1, res[dirD1], dirD1)
	}
	if !equalMappingSlice(res[dirD2], expectedD2) {
		t.Errorf("Expected mappings %v but got %v for dir %s", expectedD2, res[dirD2], dirD2)
	}
	if !equalMappingSlice(res[dirD3], expectedD3) {
		t.Errorf("Expected mappings %v but got %v for dir %s", expectedD3, res[dirD3], dirD3)
	}
}

func equalMappingSlice(a, b []Mapping) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func TestProcess(t *testing.T) {
	var groupedWork []work

	generateWorkFromSrcs := func(srcCode, srcTypeName, dstCode, dstTypeName string) work {
		srcFset := token.NewFileSet()
		srcNode, err := parser.ParseFile(srcFset, "main.go", srcCode, 0)
		if err != nil {
			t.Fatal(err)
		}

		destFset := token.NewFileSet()
		dstNode, err := parser.ParseFile(destFset, "main.go", dstCode, 0)
		if err != nil {
			t.Fatal(err)
		}

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
		g := &Generator{}
		_, err := Process(g, groupedWork)
		if err != errNoWork {
			t.Errorf("Expected error %v but got %v", errNoWork, err)
		}
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
		if err != nil {
			t.Fatal(err)
		}

		w = generateWorkFromSrcs(code2, "T", code1, "S")
		groupedWork = append(groupedWork, w)

		g := &Generator{}
		f, err := Process(g, groupedWork)
		if err != nil {
			t.Fatal(err)
		}

		if string(formattedExpectedOutput) != string(f.Buf) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(formattedExpectedOutput), string(f.Buf))
		}
	})

	t.Run("typed map", func(t *testing.T) {
		code1 := `
			package p

			type MapType map[string]any

			type StructType struct {
				Field1 string
				Field2 int
			}`

		w := generateWorkFromSrcs(code1, "MapType", code1, "StructType")
		groupedWork := []work{w}

		expectedOutput := `package p
			func (dest StructType) FromMapType(src MapType) StructType {
				if v, ok := src["Field1"].(string); ok {
					dest.Field1 = v
				}
				if v, ok := src["Field2"].(int); ok {
					dest.Field2 = v
				}
				return dest
			}`

		formattedExpectedOutput, err := format.Source([]byte(expectedOutput))
		if err != nil {
			t.Fatal(err)
		}

		g := &Generator{}
		f, err := Process(g, groupedWork)
		if err != nil {
			t.Fatal(err)
		}

		if string(formattedExpectedOutput) != string(f.Buf) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(formattedExpectedOutput), string(f.Buf))
		}
	})

	t.Run("type alias map", func(t *testing.T) {
		code1 := `
			package p

			type MapType = map[string]any

			type StructType struct {
				Field1 string
				Field2 int
			}`

		w := generateWorkFromSrcs(code1, "MapType", code1, "StructType")
		groupedWork := []work{w}

		expectedOutput := `package p
			func (dest StructType) FromMapType(src MapType) StructType {
				if v, ok := src["Field1"].(string); ok {
					dest.Field1 = v
				}
				if v, ok := src["Field2"].(int); ok {
					dest.Field2 = v
				}
				return dest
			}`

		formattedExpectedOutput, err := format.Source([]byte(expectedOutput))
		if err != nil {
			t.Fatal(err)
		}

		g := &Generator{}
		f, err := Process(g, groupedWork)
		if err != nil {
			t.Fatal(err)
		}

		if string(formattedExpectedOutput) != string(f.Buf) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(formattedExpectedOutput), string(f.Buf))
		}
	})

	t.Run("struct to typed map", func(t *testing.T) {
		code1 := `
			package p

			type MapType map[string]any

			type StructType struct {
				Field1 string
				Field2 int
			}`

		w := generateWorkFromSrcs(code1, "StructType", code1, "MapType")
		groupedWork := []work{w}

		expectedOutput := `package p
			func (dest MapType) FromStructType(src StructType) MapType {
				dest["Field1"] = src.Field1
				dest["Field2"] = src.Field2
				return dest
			}`

		formattedExpectedOutput, err := format.Source([]byte(expectedOutput))
		if err != nil {
			t.Fatal(err)
		}

		g := &Generator{}
		f, err := Process(g, groupedWork)
		if err != nil {
			t.Fatal(err)
		}

		if string(formattedExpectedOutput) != string(f.Buf) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(formattedExpectedOutput), string(f.Buf))
		}
	})

	t.Run("struct to type alias map", func(t *testing.T) {
		code1 := `
			package p

			type MapType = map[string]any

			type StructType struct {
				Field1 string
				Field2 int
			}`

		w := generateWorkFromSrcs(code1, "StructType", code1, "MapType")
		groupedWork := []work{w}

		expectedOutput := `package p
			func (dest MapType) FromStructType(src StructType) MapType {
				dest["Field1"] = src.Field1
				dest["Field2"] = src.Field2
				return dest
			}`

		formattedExpectedOutput, err := format.Source([]byte(expectedOutput))
		if err != nil {
			t.Fatal(err)
		}

		g := &Generator{}
		f, err := Process(g, groupedWork)
		if err != nil {
			t.Fatal(err)
		}

		if string(formattedExpectedOutput) != string(f.Buf) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(formattedExpectedOutput), string(f.Buf))
		}
	})

	// t.Run("recursive complex fields", func(t *testing.T) {
	// 	code1 := `
	// 		package p

	// 		type Parent struct {
	// 			Child ChildType
	// 			Name string
	// 		}

	// 		type ChildType struct {
	// 			Field1 string
	// 			Field2 int
	// 		}

	// 		type DestParent struct {
	// 			Child DestChild
	// 			Name string
	// 		}

	// 		type DestChild struct {
	// 			Field1 string
	// 			Field2 int
	// 		}
	// 	`

	// 	w := generateWorkFromSrcs(code1, "Parent", code1, "DestParent")
	// 	groupedWork := []work{w}

	// 	expectedOutput := `package p
	// 		func (dest DestParent) FromParent(src Parent) DestParent {
	// 			dest.Child.Field1 = src.Child.Field1
	// 			dest.Child.Field2 = src.Child.Field2
	// 			dest.Name = src.Name
	// 			return dest
	// 		}`

	// 	formattedExpectedOutput, err := format.Source([]byte(expectedOutput))
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}

	// 	g := Generator{}
	// 	f, err := g.Process(groupedWork)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}

	// 	if string(formattedExpectedOutput) != string(f.Buf) {
	// 		t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(formattedExpectedOutput), string(f.Buf))
	// 	}
	// })
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
	if err != nil {
		t.Fatal(err)
	}

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
		if err != nil {
			t.Fatal(err)
		}

		err = g.generate(srcData, dstData)
		if err != nil {
			t.Fatal(err)
		}

		if string(expectedOutput) != string(g.format()) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(expectedOutput), string(g.format()))
		}
	})

	t.Run("Pointer style", func(t *testing.T) {
		g := Generator{
			style: "pointer",
		}

		expectedOutput := `
			[]byte(func (dest *D) FromS(src S) {
				dest.a = src.a
			})`
		if err != nil {
			t.Fatal(err)
		}

		err = g.generate(srcData, dstData)
		if err != nil {
			t.Fatal(err)
		}

		if codeEqual(expectedOutput, string(g.format())) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(expectedOutput), string(g.format()))
		}
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
		if err != nil {
			t.Fatal(err)
		}

		err = g.generate(srcData, dstData)
		if err != nil {
			t.Fatal(err)
		}

		if string(expectedOutput) != string(g.format()) {
			t.Errorf("Expected:\n%s\n\nBut got:\n%s", string(expectedOutput), string(g.format()))
		}
	})
}

func TestGenerateMapForSource(t *testing.T) {
	type testInput struct {
		code        string
		typeName    string
		mapPlugin   MapPlugin
		style       string
		expected    string
		expectError error
	}

	runTest := func(t *testing.T, input testInput) {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "main.go", input.code, 0)
		if err != nil {
			t.Fatal(err)
		}

		g := Generator{
			style: input.style,
		}

		source := SourceData{
			node: node,
			name: input.typeName,
		}

		err = g.generateMapForSource(source, input.mapPlugin)
		if err != input.expectError {
			if err == nil {
				t.Errorf("Expected error %v but got nil", input.expectError)
			} else if input.expectError == nil {
				t.Errorf("Expected nil error but got %v", err)
			} else if err.Error() != input.expectError.Error() {
				t.Errorf("Expected error %v but got %v", input.expectError, err)
			}
		}

		if codeEqual(input.expected, string(g.format())) {
			t.Errorf("Expected output:\n%s\n\nBut got:\n%s", string(input.expected), string(g.format()))
		}
	}

	t.Run("struct to map - value style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name    string
				Age     int
				private bool
			}`

		expected := `func (dest Person) ToMap() map[string]any {
			result := make(map[string]any)
			result["Name"] = dest.Name
			result["Age"] = dest.Age
			return result
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Person",
			mapPlugin:   ToMap,
			style:       "value",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("struct to map - pointer style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name    string
				Age     int
				private bool
			}`

		expected := `
		package p
		func (dest *Person) ToMap() map[string]any {
			result := make(map[string]any)
			result["Name"] = dest.Name
			result["Age"] = dest.Age
			return result
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Person",
			mapPlugin:   ToMap,
			style:       "pointer",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("struct to map - standalone style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name    string
				Age     int
				private bool
			}`

		expected := `
		package p
		func PersonToMap(dest Person) map[string]any {
			result := make(map[string]any)
			result["Name"] = dest.Name
			result["Age"] = dest.Age
			return result
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Person",
			mapPlugin:   ToMap,
			style:       "standalone",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("map to struct - value style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name    string
				Age     int
				private bool
			}`

		expected := `
		package p
		func (dest Person) FromMap(src map[string]any) Person {
			if v, ok := src["Name"].(string); ok {
				dest.Name = v
			}
			if v, ok := src["Age"].(int); ok {
				dest.Age = v
			}
			return dest
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Person",
			mapPlugin:   FromMap,
			style:       "value",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("map to struct - pointer style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name    string
				Age     int
				private bool
			}`

		expected := `
		package p
		func (dest *Person) FromMap(src map[string]any) {
			if v, ok := src["Name"].(string); ok {
				dest.Name = v
			}
			if v, ok := src["Age"].(int); ok {
				dest.Age = v
			}
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Person",
			mapPlugin:   FromMap,
			style:       "pointer",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("map to struct - standalone style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name    string
				Age     int
				private bool
			}`

		expected := `
		package p
		func PersonFromMap(dest Person, src map[string]any) Person {
			if v, ok := src["Name"].(string); ok {
				dest.Name = v
			}
			if v, ok := src["Age"].(int); ok {
				dest.Age = v
			}
			return dest
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Person",
			mapPlugin:   FromMap,
			style:       "standalone",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("map to map type - value style", func(t *testing.T) {
		code := `
			package p

			type DataMap map[string]any`

		expected := `
		package p
		func (dest DataMap) FromMap(src map[string]any) DataMap {
			for k, v := range src {
				dest[k] = v
			}
			return dest
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "DataMap",
			mapPlugin:   FromMap,
			style:       "value",
			expected:    expected,
			expectError: nil,
		})
	})

	t.Run("map to map alias - value style", func(t *testing.T) {
		code := `
			package p

			type DataMap = map[string]any`

		expected := `
		package p
		func (dest DataMap) FromMap(src map[string]any) DataMap {
			for k, v := range src {
				dest[k] = v
			}
			return dest
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "DataMap",
			mapPlugin:   FromMap,
			style:       "value",
			expected:    expected,
			expectError: nil,
		})
	})
}

func codeEqual(got, want string) bool {
	gotFormatted, err := format.Source([]byte(got))
	if err != nil {
		return false
	}
	wantFormatted, err := format.Source([]byte(want))
	if err != nil {
		return false
	}
	return string(gotFormatted) == string(wantFormatted)
}
