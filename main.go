package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var typeName string
	var outputPath string
	var workingDirectory string

	flag.StringVar(&typeName, "type", "", "Name of the interface type to generate mapper for")
	flag.StringVar(&outputPath, "output", "", "Output file path (default: km_{type}_gen.go)")
	flag.StringVar(&workingDirectory, "workdir", ".", "working directory")
	flag.Parse()

	if typeName == "" {
		fail("-type is required")
	}

	if outputPath == "" {
		outputPath = fmt.Sprintf("km_%s_gen.go", strings.ToLower(typeName))
	}

	config := GeneratorConfig{
		WorkingDir:    workingDirectory,
		OutputFile:    outputPath,
		InterfaceName: typeName,
	}

	if err := Generate(config); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "km: %s\n", message)
	os.Exit(1)
}
