package main

import (
	"bytes"
	"fmt"
	"go/format"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
)

type Generator struct {
	buf            bytes.Buffer
	style          string
	module         string
	pathFromModule string
}

func (g *Generator) generate(source SourceData, destination DestinationData) error {
	sourceFields, err := getFields(source.node, source.name)
	if err != nil {
		return err
	}
	destinationFields, err := getFields(destination.node, destination.name)
	if err != nil {
		return err
	}

	sourceFieldsLookup := make(map[string]Field)
	for _, f := range sourceFields {
		sourceFieldsLookup[f.Name] = f
	}

	var srcName string

	samePkg := filepath.Dir(source.path) == filepath.Dir(destination.path)

	if samePkg {
		srcName = source.name
	} else {
		srcName = fmt.Sprintf("%s.%s", getPackage(source.node), source.name)
	}

	switch g.style {
	case "pointer":
		g.Printf("func (dest *%s) From%s(src %s) {", destination.name, source.name, srcName)
	case "value", "":
		g.Printf("func (dest %s) From%s(src %s) %s {", destination.name, source.name, srcName, destination.name)
	case "standalone":
		g.Printf("func %sFrom%s(dest %s, src %s) %s {", destination.name, source.name, destination.name, srcName, destination.name)
	}

	for _, destinationField := range destinationFields {
		_, ignored := destination.ignoredMap[destinationField.Name]
		isExported := unicode.IsUpper(rune(destinationField.Name[0]))
		if ignored || (!isExported && !samePkg) {
			continue
		}

		sourceField, ok := sourceFieldsLookup[destinationField.Name]
		if !ok {
			sourceField, ok = sourceFieldsLookup[destination.fieldsMap[destinationField.Name]]
		}
		if ok {
			// NOTE(khatibomar): I should support convertion between convertible types
			isImportedFromSource := strings.TrimPrefix(sourceField.Type, fmt.Sprintf("%s.", getPackage(destination.node))) == destinationField.Type &&
				in(joinLinuxPath(g.module, g.pathFromModule, filepath.Dir(destination.path)), getImports(source.node))
			isImportedFromDestination := strings.TrimPrefix(destinationField.Type, fmt.Sprintf("%s.", getPackage(source.node))) == sourceField.Type &&
				in(joinLinuxPath(g.module, g.pathFromModule, filepath.Dir(source.path)), getImports(destination.node))
			if sourceField.Type == destinationField.Type || isImportedFromSource || isImportedFromDestination {
				g.Printf("dest.%s = src.%s\n", destinationField.Name, sourceField.Name)
			}
		}
	}

	switch g.style {
	case "pointer":
		g.Printf("}")
	case "value", "standalone", "":
		g.Printf("return dest }")
	}

	return nil
}

func (g *Generator) Printf(format string, args ...any) {
	fmt.Fprintf(&g.buf, format, args...)
}

func (g *Generator) format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Warn().
			Msg(fmt.Sprintf("internal error: invalid Go generated: %s", err))
		log.Warn().
			Msg("compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}

func (g *Generator) Process(groupedWork []work) (File, error) {
	var result File

	if len(groupedWork) == 0 {
		return result, errNoWork
	}

	g.Printf("package %s\n", getPackage(groupedWork[0].Destination.node))

	var imports []string

	for _, w := range groupedWork {
		samePkg := filepath.Dir(w.Source.path) == filepath.Dir(w.Destination.path)
		if !samePkg {
			imports = append(imports, fmt.Sprintf("\"%s\"\n", joinLinuxPath(g.module, g.pathFromModule, filepath.Dir(w.Source.path))))
		}
	}

	if len(imports) > 0 {
		g.Printf("import (\n")
		g.Printf(strings.Join(imports, "\n"))
		g.Printf(")\n")
	}

	for _, w := range groupedWork {
		if err := g.generate(w.Source, w.Destination); err != nil {
			return result, err
		}
		g.Printf("\n\n")
	}

	return File{
		Path: filepath.Dir(groupedWork[0].Destination.path),
		Buf:  g.format(),
	}, nil
}
