package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
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

	if isMapType(destination.node, destination.name) {
		for _, srcField := range sourceFields {
			g.Printf("dest[\"%s\"] = src.%s\n", srcField.Name, srcField.Name)
		}
		goto functionCloser
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

		if isMapType(source.node, source.name) {
			g.Printf("if v, ok := src[\"%s\"].(%s); ok {\n", destinationField.Name, destinationField.Type)
			g.Printf("dest.%s = v\n", destinationField.Name)
			g.Printf("}\n")
			continue
		}

		if ok {
			isImportedFromSource := strings.TrimPrefix(sourceField.Type, fmt.Sprintf("%s.", getPackage(destination.node))) == destinationField.Type &&
				in(joinLinuxPath(g.module, g.pathFromModule, filepath.Dir(destination.path)), getImports(source.node))
			isImportedFromDestination := strings.TrimPrefix(destinationField.Type, fmt.Sprintf("%s.", getPackage(source.node))) == sourceField.Type &&
				in(joinLinuxPath(g.module, g.pathFromModule, filepath.Dir(source.path)), getImports(destination.node))

			if sourceField.Type == destinationField.Type || isImportedFromSource || isImportedFromDestination {
				g.Printf("dest.%s = src.%s\n", destinationField.Name, sourceField.Name)
			} else if areTypesConvertible(sourceField.Type, destinationField.Type) {
				g.Printf("dest.%s = %s(src.%s)\n",
					destinationField.Name,
					destinationField.Type,
					sourceField.Name)
			}
		}
	}

functionCloser:
	switch g.style {
	case "pointer":
		g.Printf("}")
	case "value", "standalone", "":
		g.Printf("return dest }")
	}

	// NOTE(khatibomar): revist this later, maybe we need it in some edge cases
	// like having more code to be generated after this method
	// so we need breaking lines at end.
	// g.Printf("\n\n")

	return nil
}

func (g *Generator) generateMapForSource(source SourceData, plugin MapPlugin) error {
	fields, err := getFields(source.node, source.name)
	if err != nil {
		return err
	}

	switch g.style {
	case "pointer":
		if plugin == FromMap {
			g.Printf("func (dest *%s) FromMap(src map[string]any) {", source.name)
		} else {
			g.Printf("func (dest *%s) ToMap() map[string]any {", source.name)
		}
	case "value", "":
		if plugin == FromMap {
			g.Printf("func (dest %s) FromMap(src map[string]any) %s {", source.name, source.name)
		} else {
			g.Printf("func (dest %s) ToMap() map[string]any {", source.name)
		}
	case "standalone":
		if plugin == FromMap {
			g.Printf("func %sFromMap(dest %s, src map[string]any) %s {", source.name, source.name, source.name)
		} else {
			g.Printf("func %sToMap(dest %s) map[string]any {", source.name, source.name)
		}
	}

	if plugin == ToMap {
		g.Printf("result := make(map[string]any)\n")
		for _, field := range fields {
			if unicode.IsUpper(rune(field.Name[0])) {
				g.Printf("result[\"%s\"] = dest.%s\n", field.Name, field.Name)
			}
		}
	} else {
		if isMapType(source.node, source.name) {
			for _, field := range fields {
				g.Printf("dest[\"%s\"] = src[\"%s\"]\n", field.Name, field.Name)
			}
		} else {
			for _, field := range fields {
				if unicode.IsUpper(rune(field.Name[0])) {
					g.Printf("if v, ok := src[\"%s\"].(%s); ok {\n", field.Name, field.Type)
					g.Printf("dest.%s = v\n", field.Name)
					g.Printf("}\n")
				}
			}
		}
	}

	if plugin == ToMap {
		g.Printf("return result }")
	} else {
		switch g.style {
		case "pointer":
			g.Printf("}")
		case "value", "standalone", "":
			g.Printf("return dest }")
		}
	}

	g.Printf("\n\n")

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
		defaultLogger.Warning("internal error: invalid go generated: %s", err)
		defaultLogger.Warning("compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}

func Process[T workTyper](g *Generator, groupedWork []T) (File, error) {
	var result File

	if len(groupedWork) == 0 {
		return result, errNoWork
	}

	if version != "test" {
		g.Printf("// Code generated by KM; DO NOT EDIT.\n")
		g.Printf("// Generated at: %s\n", time.Now().Format(time.RFC3339))
		g.Printf("// KM version: %s\n\n", version)
	}

	firstWork := groupedWork[0]

	if srcWork, ok := any(firstWork).(mapWork); ok {
		g.Printf("package %s\n", getPackage(srcWork.Target.node))

		for _, w := range groupedWork {
			if wt, ok := any(w).(mapWork); ok {
				if err := g.generateMapForSource(wt.Target, wt.plugin); err != nil {
					return result, err
				}
			}
		}

		return File{
			Path: filepath.Dir(srcWork.Target.path),
			Buf:  g.format(),
		}, nil
	}

	regularWork, ok := any(firstWork).(work)
	if !ok {
		return result, fmt.Errorf("unsupported work type: %T", firstWork)
	}

	g.Printf("package %s\n", getPackage(regularWork.Destination.node))

	var imports []string

	for _, w := range groupedWork {
		if wt, ok := any(w).(work); ok {
			samePkg := filepath.Dir(wt.Source.path) == filepath.Dir(wt.Destination.path)
			if !samePkg {
				imports = append(imports, fmt.Sprintf("\"%s\"\n", joinLinuxPath(g.module, g.pathFromModule, filepath.Dir(wt.Source.path))))
			}
		}
	}

	if len(imports) > 0 {
		g.Printf("import (\n")
		g.Printf("%s", strings.Join(imports, "\n"))
		g.Printf(")\n")
	}

	for _, w := range groupedWork {
		if wt, ok := any(w).(work); ok {
			if err := g.generate(wt.Source, wt.Destination); err != nil {
				return result, err
			}
			g.Printf("\n\n")
		}
	}

	return File{
		Path: filepath.Dir(regularWork.Destination.path),
		Buf:  g.format(),
	}, nil
}

func isMapType(node *ast.File, typeName string) bool {
	isMap := false
	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.Name == typeName {
				if _, ok := typeSpec.Type.(*ast.MapType); ok {
					isMap = true
					return false
				}
			}
		}
		return true
	})
	return isMap
}

func areTypesConvertible(sourceType, destType string) bool {
	basicTypes := map[string][]string{
		"int":     {"int32", "int64", "float64", "string"},
		"int32":   {"int", "int64", "float64", "string"},
		"int64":   {"int", "int32", "float64", "string"},
		"float64": {"int", "int32", "int64", "string"},
		"string":  {"int", "int32", "int64", "float64"},
	}

	if convertibleTypes, ok := basicTypes[sourceType]; ok {
		return slices.Contains(convertibleTypes, destType)
	}
	return false
}
