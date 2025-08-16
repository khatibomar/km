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

	t.Run("nested complex types", func(t *testing.T) {
		srcCode := `
			package p

			type User struct {
				Name string
				Details UserDetails
				Settings *UserSettings
				Friends []Friend
			}

			type UserDetails struct {
				Age int
				Address Address
			}

			type Address struct {
				Street string
				City string
			}

			type UserSettings struct {
				Theme string
				Language string
			}

			type Friend struct {
				Name string
				Since int
			}
		`

		dstCode := `
			package q

			type Customer struct {
				Name string
				Details CustomerInfo
				Config *CustomerConfig
				Contacts []Contact
			}

			type CustomerInfo struct {
				Age int
				Location Location
			}

			type Location struct {
				Street string
				City string
			}

			type CustomerConfig struct {
				Theme string
				Language string
			}

			type Contact struct {
				Name string
				Since int
			}
		`

		expectedOutput := `func (dest Customer) FromUser(src p.User) Customer {
			dest.Name = src.Name
			dest.Details = dest.Details.FromUserDetails(src.Details)
			if src.Settings != nil {
				if dest.Config == nil {
					dest.Config = &CustomerConfig{}
				}
				dest.Config = dest.Config.FromUserSettings(*src.Settings)
			}
			dest.Contacts = make([]Contact, len(src.Friends))
			for i, f := range src.Friends {
				dest.Contacts[i] = dest.Contacts[i].FromFriend(f)
			}
			return dest
		}

		func (dest CustomerInfo) FromUserDetails(src p.UserDetails) CustomerInfo {
			dest.Age = src.Age
			dest.Location = dest.Location.FromAddress(src.Address)
			return dest
		}

		func (dest Location) FromAddress(src p.Address) Location {
			dest.Street = src.Street
			dest.City = src.City
			return dest
		}

		func (dest *CustomerConfig) FromUserSettings(src p.UserSettings) *CustomerConfig {
			dest.Theme = src.Theme
			dest.Language = src.Language
			return dest
		}

		func (dest Contact) FromFriend(src p.Friend) Contact {
			dest.Name = src.Name
			dest.Since = src.Since
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/pkg/p/user.go",
			destPath:              "/pkg/q/customer.go",
			srcName:               "User",
			destName:              "Customer",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
			fieldsMap: map[string]string{
				"Config":   "Settings",
				"Contacts": "Friends",
				"Location": "Address",
			},
		})
	})

	t.Run("anonymous struct fields", func(t *testing.T) {
		srcCode := `
			package p

			type Source struct {
				string
				int
				Data struct {
					Value float64
				}
				Meta struct {
					Tags []string
				}
			}`

		dstCode := `
			package q

			type Dest struct {
				string
				int
				Data struct {
					Value float64
				}
				Meta struct {
					Tags []string
				}
			}`

		expectedOutput := `func (dest Dest) FromSource(src p.Source) Dest {
			dest.string = src.string
			dest.int = src.int
			dest.Data.Value = src.Data.Value
			dest.Meta.Tags = append([]string{}, src.Meta.Tags...)
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/p/source.go",
			destPath:              "/q/dest.go",
			srcName:               "Source",
			destName:              "Dest",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("embedded struct types", func(t *testing.T) {
		srcCode := `
			package p

			type Base struct {
				ID int
				CreatedAt string
			}

			type Source struct {
				Base
				Name string
				Details struct {
					Base
					Extra string
				}
			}`

		dstCode := `
			package q

			type DestBase struct {
				ID int
				CreatedAt string
			}

			type Dest struct {
				DestBase
				Name string
				Details struct {
					DestBase
					Extra string
				}
			}`

		expectedOutput := `func (dest Dest) FromSource(src p.Source) Dest {
			dest.ID = src.ID
			dest.CreatedAt = src.CreatedAt
			dest.Name = src.Name
			dest.Details.ID = src.Details.ID
			dest.Details.CreatedAt = src.Details.CreatedAt
			dest.Details.Extra = src.Details.Extra
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/p/source.go",
			destPath:              "/q/dest.go",
			srcName:               "Source",
			destName:              "Dest",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
		})
	})

	t.Run("generic types", func(t *testing.T) {
		srcCode := `
			package p

			type Box[T any] struct {
				Value T
				Label string
			}

			type StringBox struct {
				Value string
				Label string
			}
		`

		dstCode := `
			package q

			type Container[T any] struct {
				Value T
				Name string
			}

			type StringContainer struct {
				Value string
				Name string
			}
		`

		expectedOutput := `func (dest StringContainer) FromStringBox(src p.StringBox) StringContainer {
			dest.Value = src.Value
			dest.Name = src.Label
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/p/box.go",
			destPath:              "/q/container.go",
			srcName:               "StringBox",
			destName:              "StringContainer",
			expectedOutput:        expectedOutput,
			expectedGenerateError: nil,
			fieldsMap: map[string]string{
				"Name": "Label",
			},
		})
	})

	t.Run("same type names different packages", func(t *testing.T) {
		srcCode := `
			package foo

			type User struct {
				ID int
				Name string
			}
		`

		dstCode := `
			package bar

			type User struct {
				ID int
				Name string
			}
		`

		expectedOutput := `func (dest User) FromUser(src foo.User) User {
			dest.ID = src.ID
			dest.Name = src.Name
			return dest
		}`

		runTest(t, testInput{
			srcCode:               srcCode,
			destCode:              dstCode,
			sourcePath:            "/foo/user.go",
			destPath:              "/bar/user.go",
			srcName:               "User",
			destName:              "User",
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

	t.Run("nested struct to map - value style", func(t *testing.T) {
		code := `
			package p

			type Address struct {
				Street string
				City   string
			}

			type Person struct {
				Name    string
				Age     int
				Address Address
			}`

		expected := `
		package p
		func (dest Person) ToMap() map[string]any {
			result := make(map[string]any)
			result["Name"] = dest.Name
			result["Age"] = dest.Age
			address := make(map[string]any)
			address["Street"] = dest.Address.Street
			address["City"] = dest.Address.City
			result["Address"] = address
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

	t.Run("struct with slices to map - value style", func(t *testing.T) {
		code := `
			package p

			type Person struct {
				Name     string
				Hobbies  []string
				Scores   []int
			}`

		expected := `
		package p
		func (dest Person) ToMap() map[string]any {
			result := make(map[string]any)
			result["Name"] = dest.Name
			result["Hobbies"] = dest.Hobbies
			result["Scores"] = dest.Scores
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

	t.Run("struct with custom types to map - value style", func(t *testing.T) {
		code := `
			package p

			type CustomID int
			type Status string

			type Record struct {
				ID     CustomID
				State  Status
			}`

		expected := `
		package p
		func (dest Record) ToMap() map[string]any {
			result := make(map[string]any)
			result["ID"] = dest.ID
			result["State"] = dest.State
			return result
		}`

		runTest(t, testInput{
			code:        code,
			typeName:    "Record",
			mapPlugin:   ToMap,
			style:       "value",
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
