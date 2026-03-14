package main

import (
	"flag"
	"fmt"
	"os"
)

type mappingFlag []string

func (m *mappingFlag) String() string {
	if m == nil {
		return ""
	}

	return fmt.Sprintf("%v", []string(*m))
}

func (m *mappingFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	var mappings mappingFlag
	var outputPath string
	var packageName string
	var packagePath string
	var workingDirectory string

	flag.Var(&mappings, "mapping", "mapping rule in the form source=destination[#FunctionName]")
	flag.StringVar(&outputPath, "output", "", "output file path for generated mappers")
	flag.StringVar(&packageName, "package", "", "package name for generated file")
	flag.StringVar(&packagePath, "package-path", "", "import path of the generated package")
	flag.StringVar(&workingDirectory, "workdir", ".", "working directory used for package loading")
	flag.Parse()

	if len(mappings) == 0 {
		fail("at least one -mapping must be provided")
	}
	if outputPath == "" {
		fail("-output is required")
	}
	if packageName == "" {
		fail("-package is required")
	}
	if packagePath == "" {
		fail("-package-path is required")
	}

	specs := make([]MappingSpec, 0, len(mappings))
	for _, raw := range mappings {
		spec, err := ParseMappingSpec(raw)
		if err != nil {
			fail(err.Error())
		}
		specs = append(specs, spec)
	}

	config := GeneratorConfig{
		WorkingDir:  workingDirectory,
		OutputFile:  outputPath,
		PackageName: packageName,
		PackagePath: packagePath,
		Mappings:    specs,
	}

	if err := Generate(config); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "km: %s\n", message)
	os.Exit(1)
}
