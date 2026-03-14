package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMappingSpec(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		expects MappingSpec
	}{
		{
			name:  "with explicit function",
			input: "example.com/a/source.User=example.com/a/destination.UserDTO#MapUser",
			expects: MappingSpec{
				Source:      "example.com/a/source.User",
				Destination: "example.com/a/destination.UserDTO",
				Function:    "MapUser",
			},
		},
		{
			name:  "without explicit function",
			input: "example.com/a/source.User=example.com/a/destination.UserDTO",
			expects: MappingSpec{
				Source:      "example.com/a/source.User",
				Destination: "example.com/a/destination.UserDTO",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := ParseMappingSpec(testCase.input)
			if err != nil {
				t.Fatalf("ParseMappingSpec returned error: %v", err)
			}

			if result != testCase.expects {
				t.Fatalf("unexpected parse result: got %#v, want %#v", result, testCase.expects)
			}
		})
	}
}

func TestGenerateCompileTimeMapperWithNestedAndGenerics(t *testing.T) {
	moduleDirectory := t.TempDir()
	modulePath := "example.com/kmgenit"

	writeFile(t, filepath.Join(moduleDirectory, "go.mod"), "module "+modulePath+"\n\ngo 1.25.0\n")

	writeFile(t, filepath.Join(moduleDirectory, "source", "types.go"), `package source

type Box[T any] struct {
	Value T
	Next  *Box[T]
	Items []T
	Meta  map[string]T
}

type Child struct {
	ID    int
	Notes []string
}

type Envelope struct {
	Name    string
	Count   int
	Child   *Child
	Matrix  [][]int
	Payload Box[int]
	Bucket  map[string]Box[int]
}
`)

	writeFile(t, filepath.Join(moduleDirectory, "destination", "types.go"), `package destination

type Box[T any] struct {
	Value T
	Next  *Box[T]
	Items []T
	Meta  map[string]T
}

type ChildDTO struct {
	ID    int64
	Notes []string
}

type EnvelopeDTO struct {
	Name    string
	Count   int64
	Child   *ChildDTO
	Matrix  [][]int64
	Payload Box[int64]
	Bucket  map[string]Box[int64]
}
`)

	outputFile := filepath.Join(moduleDirectory, "mappers", "mapper_gen.go")
	err := Generate(GeneratorConfig{
		WorkingDir:  moduleDirectory,
		OutputFile:  outputFile,
		PackageName: "mappers",
		PackagePath: modulePath + "/mappers",
		Mappings: []MappingSpec{
			{
				Source:      modulePath + "/source.Envelope",
				Destination: modulePath + "/destination.EnvelopeDTO",
				Function:    "EnvelopeToEnvelopeDTO",
			},
		},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	writeFile(t, filepath.Join(moduleDirectory, "mappers", "mapper_gen_test.go"), `package mappers

import (
	"testing"

	"example.com/kmgenit/source"
)

func TestEnvelopeToEnvelopeDTO(t *testing.T) {
	nextPayload := source.Box[int]{
		Value: 11,
		Items: []int{7, 8},
		Meta:  map[string]int{"x": 9},
	}

	in := source.Envelope{
		Name:  "demo",
		Count: 3,
		Child: &source.Child{ID: 4, Notes: []string{"a", "b"}},
		Matrix: [][]int{
			{1, 2},
			{3, 4},
		},
		Payload: source.Box[int]{
			Value: 10,
			Next:  &nextPayload,
			Items: []int{5, 6},
			Meta:  map[string]int{"k": 1},
		},
		Bucket: map[string]source.Box[int]{
			"main": {
				Value: 20,
				Items: []int{30, 40},
				Meta:  map[string]int{"z": 50},
			},
		},
	}

	out := EnvelopeToEnvelopeDTO(in)

	if out.Name != "demo" {
		t.Fatalf("unexpected name: %s", out.Name)
	}
	if out.Count != int64(3) {
		t.Fatalf("unexpected count: %d", out.Count)
	}
	if out.Child == nil || out.Child.ID != int64(4) {
		t.Fatalf("unexpected child conversion")
	}
	if out.Payload.Next == nil || out.Payload.Next.Value != int64(11) {
		t.Fatalf("unexpected nested pointer conversion")
	}
	if len(out.Matrix) != 2 || out.Matrix[0][1] != int64(2) {
		t.Fatalf("unexpected matrix conversion")
	}
	if out.Bucket["main"].Items[1] != int64(40) {
		t.Fatalf("unexpected map/generic conversion")
	}
}
`)

	command := exec.Command("go", "test", "./...")
	command.Dir = moduleDirectory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated module tests failed: %v\n%s", err, string(output))
	}
}

func TestGenerateFailsForUnsupportedTypes(t *testing.T) {
	moduleDirectory := t.TempDir()
	modulePath := "example.com/kmgenfail"

	writeFile(t, filepath.Join(moduleDirectory, "go.mod"), "module "+modulePath+"\n\ngo 1.25.0\n")
	writeFile(t, filepath.Join(moduleDirectory, "source", "types.go"), `package source

type User struct {
	Age int
}
`)
	writeFile(t, filepath.Join(moduleDirectory, "destination", "types.go"), `package destination

type UserDTO struct {
	Age []int
}
`)

	err := Generate(GeneratorConfig{
		WorkingDir:  moduleDirectory,
		OutputFile:  filepath.Join(moduleDirectory, "mappers", "mapper_gen.go"),
		PackageName: "mappers",
		PackagePath: modulePath + "/mappers",
		Mappings: []MappingSpec{
			{
				Source:      modulePath + "/source.User",
				Destination: modulePath + "/destination.UserDTO",
				Function:    "UserToUserDTO",
			},
		},
	})
	if err == nil {
		t.Fatalf("expected generation error for incompatible field types")
	}
	if !strings.Contains(err.Error(), "unsupported mapping") {
		t.Fatalf("expected unsupported mapping error, got: %v", err)
	}
}

func writeFile(t *testing.T, filePath string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(filePath), err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}
}
