package main

import (
	"fmt"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

type MappingSpec struct {
	Source      string
	Destination string
	Function    string
}

type GeneratorConfig struct {
	WorkingDir  string
	OutputFile  string
	PackageName string
	PackagePath string
	Mappings    []MappingSpec
}

type parsedTypeSpec struct {
	PackagePath string
	TypeExpr    string
}

type resolvedType struct {
	parsed parsedTypeSpec
	typ    types.Type
}

type resolvedMapping struct {
	spec        MappingSpec
	source      resolvedType
	destination resolvedType
	function    string
	helper      string
}

type helperDefinition struct {
	name        string
	source      types.Type
	destination types.Type
	generated   bool
}

type codeGenerator struct {
	config          GeneratorConfig
	imports         *importRegistry
	resolvedMapping []resolvedMapping
	helpers         map[string]*helperDefinition
	helperQueue     []*helperDefinition
	helperCounter   int
	tempCounter     int
}

func ParseMappingSpec(raw string) (MappingSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return MappingSpec{}, fmt.Errorf("empty mapping value")
	}

	mapping := MappingSpec{}
	body := trimmed
	if strings.Contains(trimmed, "#") {
		parts := strings.SplitN(trimmed, "#", 2)
		body = strings.TrimSpace(parts[0])
		mapping.Function = strings.TrimSpace(parts[1])
	}

	pair := strings.SplitN(body, "=", 2)
	if len(pair) != 2 {
		return MappingSpec{}, fmt.Errorf("invalid mapping %q: expected source=destination[#FunctionName]", raw)
	}

	mapping.Source = strings.TrimSpace(pair[0])
	mapping.Destination = strings.TrimSpace(pair[1])
	if mapping.Source == "" || mapping.Destination == "" {
		return MappingSpec{}, fmt.Errorf("invalid mapping %q: source and destination are required", raw)
	}

	if mapping.Function != "" && !isValidIdentifier(mapping.Function) {
		return MappingSpec{}, fmt.Errorf("invalid function name %q", mapping.Function)
	}

	return mapping, nil
}

func Generate(config GeneratorConfig) error {
	generator, err := newCodeGenerator(config)
	if err != nil {
		return err
	}

	content, err := generator.build()
	if err != nil {
		return err
	}

	outputDirectory := filepath.Dir(config.OutputFile)
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(config.OutputFile, content, 0o644); err != nil {
		return fmt.Errorf("write generated file: %w", err)
	}

	return nil
}

func newCodeGenerator(config GeneratorConfig) (*codeGenerator, error) {
	if config.WorkingDir == "" {
		config.WorkingDir = "."
	}
	if config.OutputFile == "" {
		return nil, fmt.Errorf("output file is required")
	}
	if config.PackageName == "" {
		return nil, fmt.Errorf("package name is required")
	}
	if !isValidIdentifier(config.PackageName) {
		return nil, fmt.Errorf("invalid package name %q", config.PackageName)
	}
	if config.PackagePath == "" {
		return nil, fmt.Errorf("package path is required")
	}
	if len(config.Mappings) == 0 {
		return nil, fmt.Errorf("at least one mapping is required")
	}

	parsedMappings := make([]struct {
		spec       MappingSpec
		sourceSpec parsedTypeSpec
		destSpec   parsedTypeSpec
	}, 0, len(config.Mappings))

	packageSet := map[string]struct{}{}
	for _, mapping := range config.Mappings {
		sourceSpec, err := parseTypeSpec(mapping.Source)
		if err != nil {
			return nil, fmt.Errorf("invalid source %q: %w", mapping.Source, err)
		}
		destSpec, err := parseTypeSpec(mapping.Destination)
		if err != nil {
			return nil, fmt.Errorf("invalid destination %q: %w", mapping.Destination, err)
		}

		parsedMappings = append(parsedMappings, struct {
			spec       MappingSpec
			sourceSpec parsedTypeSpec
			destSpec   parsedTypeSpec
		}{
			spec:       mapping,
			sourceSpec: sourceSpec,
			destSpec:   destSpec,
		})

		packageSet[sourceSpec.PackagePath] = struct{}{}
		packageSet[destSpec.PackagePath] = struct{}{}
	}

	packagePaths := make([]string, 0, len(packageSet))
	for packagePath := range packageSet {
		packagePaths = append(packagePaths, packagePath)
	}
	sort.Strings(packagePaths)

	loadedPackages, err := loadPackages(config.WorkingDir, packagePaths)
	if err != nil {
		return nil, err
	}

	generator := &codeGenerator{
		config:  config,
		imports: newImportRegistry(config.PackageName, config.PackagePath),
		helpers: map[string]*helperDefinition{},
	}

	generator.resolvedMapping = make([]resolvedMapping, 0, len(parsedMappings))
	for _, parsedMapping := range parsedMappings {
		sourceType, err := resolveType(loadedPackages, parsedMapping.sourceSpec)
		if err != nil {
			return nil, fmt.Errorf("resolve source %q: %w", parsedMapping.spec.Source, err)
		}
		destinationType, err := resolveType(loadedPackages, parsedMapping.destSpec)
		if err != nil {
			return nil, fmt.Errorf("resolve destination %q: %w", parsedMapping.spec.Destination, err)
		}

		functionName := parsedMapping.spec.Function
		if functionName == "" {
			functionName = defaultFunctionName(parsedMapping.sourceSpec.TypeExpr, parsedMapping.destSpec.TypeExpr)
		}
		if !isValidIdentifier(functionName) {
			return nil, fmt.Errorf("invalid function name %q", functionName)
		}

		generator.resolvedMapping = append(generator.resolvedMapping, resolvedMapping{
			spec:        parsedMapping.spec,
			source:      sourceType,
			destination: destinationType,
			function:    functionName,
		})
	}

	functionSet := map[string]struct{}{}
	for _, mapping := range generator.resolvedMapping {
		if _, exists := functionSet[mapping.function]; exists {
			return nil, fmt.Errorf("duplicate function name %q", mapping.function)
		}
		functionSet[mapping.function] = struct{}{}
	}

	return generator, nil
}

func (g *codeGenerator) build() ([]byte, error) {
	for index := range g.resolvedMapping {
		helperName, err := g.ensureHelper(g.resolvedMapping[index].source.typ, g.resolvedMapping[index].destination.typ)
		if err != nil {
			return nil, err
		}
		g.resolvedMapping[index].helper = helperName
	}

	helperWriter := &codeWriter{}
	for queueIndex := 0; queueIndex < len(g.helperQueue); queueIndex++ {
		helper := g.helperQueue[queueIndex]
		if helper.generated {
			continue
		}
		if err := g.emitHelper(helperWriter, helper); err != nil {
			return nil, err
		}
		helper.generated = true
	}

	wrapperWriter := &codeWriter{}
	for _, mapping := range g.resolvedMapping {
		g.emitWrapper(wrapperWriter, mapping)
	}

	finalWriter := &codeWriter{}
	finalWriter.line("// Code generated by km. DO NOT EDIT.")
	finalWriter.linef("package %s", g.config.PackageName)
	finalWriter.line("")

	imports := g.imports.declarations()
	if len(imports) > 0 {
		finalWriter.line("import (")
		finalWriter.indent++
		for _, declaration := range imports {
			finalWriter.linef("%s %q", declaration.alias, declaration.path)
		}
		finalWriter.indent--
		finalWriter.line(")")
		finalWriter.line("")
	}

	finalWriter.line(wrapperWriter.String())
	if wrapperWriter.Len() > 0 && helperWriter.Len() > 0 {
		finalWriter.line("")
	}
	finalWriter.line(helperWriter.String())

	formatted, err := format.Source([]byte(finalWriter.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w\n%s", err, finalWriter.String())
	}

	return formatted, nil
}

func (g *codeGenerator) emitWrapper(writer *codeWriter, mapping resolvedMapping) {
	sourceType := g.renderType(mapping.source.typ)
	destinationType := g.renderType(mapping.destination.typ)

	writer.linef("func %s(in %s) %s {", mapping.function, sourceType, destinationType)
	writer.indent++
	writer.linef("return %s(in)", mapping.helper)
	writer.indent--
	writer.line("}")
	writer.line("")
}

func (g *codeGenerator) ensureHelper(source types.Type, destination types.Type) (string, error) {
	if hasUnboundTypeParam(source) {
		return "", fmt.Errorf("source type %s has unbound type parameters", typeKey(source))
	}
	if hasUnboundTypeParam(destination) {
		return "", fmt.Errorf("destination type %s has unbound type parameters", typeKey(destination))
	}

	key := typePairKey(source, destination)
	if helper, exists := g.helpers[key]; exists {
		return helper.name, nil
	}

	g.helperCounter++
	helperName := fmt.Sprintf("mapType%d", g.helperCounter)
	helper := &helperDefinition{
		name:        helperName,
		source:      source,
		destination: destination,
	}

	g.helpers[key] = helper
	g.helperQueue = append(g.helperQueue, helper)
	return helperName, nil
}

func (g *codeGenerator) emitHelper(writer *codeWriter, helper *helperDefinition) error {
	sourceType := g.renderType(helper.source)
	destinationType := g.renderType(helper.destination)

	writer.linef("func %s(in %s) %s {", helper.name, sourceType, destinationType)
	writer.indent++

	if err := g.emitHelperBody(writer, helper.source, helper.destination); err != nil {
		return err
	}

	writer.indent--
	writer.line("}")
	writer.line("")
	return nil
}

func (g *codeGenerator) emitHelperBody(writer *codeWriter, source types.Type, destination types.Type) error {
	if types.AssignableTo(source, destination) {
		writer.line("return in")
		return nil
	}

	if g.isBasicConvertible(source, destination) {
		writer.linef("return %s(in)", g.renderType(destination))
		return nil
	}

	sourceUnaliased := types.Unalias(source)
	destinationUnaliased := types.Unalias(destination)
	sourceUnderlying := sourceUnaliased.Underlying()
	destinationUnderlying := destinationUnaliased.Underlying()

	if destinationPointer, destinationIsPointer := destinationUnderlying.(*types.Pointer); destinationIsPointer {
		if sourcePointer, sourceIsPointer := sourceUnderlying.(*types.Pointer); sourceIsPointer {
			elemHelper, err := g.ensureHelper(sourcePointer.Elem(), destinationPointer.Elem())
			if err != nil {
				return err
			}

			writer.line("if in == nil {")
			writer.indent++
			writer.line("return nil")
			writer.indent--
			writer.line("}")
			writer.linef("out := new(%s)", g.renderType(destinationPointer.Elem()))
			writer.linef("*out = %s(*in)", elemHelper)
			writer.line("return out")
			return nil
		}

		elemHelper, err := g.ensureHelper(source, destinationPointer.Elem())
		if err != nil {
			return err
		}

		writer.linef("out := new(%s)", g.renderType(destinationPointer.Elem()))
		writer.linef("*out = %s(in)", elemHelper)
		writer.line("return out")
		return nil
	}

	if sourcePointer, sourceIsPointer := sourceUnderlying.(*types.Pointer); sourceIsPointer {
		elemHelper, err := g.ensureHelper(sourcePointer.Elem(), destination)
		if err != nil {
			return err
		}

		writer.line("if in == nil {")
		writer.indent++
		writer.linef("var zero %s", g.renderType(destination))
		writer.line("return zero")
		writer.indent--
		writer.line("}")
		writer.linef("return %s(*in)", elemHelper)
		return nil
	}

	switch typedDestination := destinationUnderlying.(type) {
	case *types.Struct:
		typedSource, ok := sourceUnderlying.(*types.Struct)
		if !ok {
			return fmt.Errorf("unsupported mapping %s -> %s", g.renderType(source), g.renderType(destination))
		}

		writer.linef("var out %s", g.renderType(destination))
		for fieldIndex := 0; fieldIndex < typedDestination.NumFields(); fieldIndex++ {
			destinationField := typedDestination.Field(fieldIndex)
			if !destinationField.Exported() {
				continue
			}

			sourceField, ok := findExportedFieldByName(typedSource, destinationField.Name())
			if !ok {
				continue
			}

			fieldTarget := fmt.Sprintf("out.%s", destinationField.Name())
			fieldSource := fmt.Sprintf("in.%s", sourceField.Name())

			if types.AssignableTo(sourceField.Type(), destinationField.Type()) {
				writer.linef("%s = %s", fieldTarget, fieldSource)
				continue
			}
			if g.isBasicConvertible(sourceField.Type(), destinationField.Type()) {
				writer.linef("%s = %s(%s)", fieldTarget, g.renderType(destinationField.Type()), fieldSource)
				continue
			}

			fieldHelper, err := g.ensureHelper(sourceField.Type(), destinationField.Type())
			if err != nil {
				return err
			}
			writer.linef("%s = %s(%s)", fieldTarget, fieldHelper, fieldSource)
		}
		writer.line("return out")
		return nil
	case *types.Slice:
		destinationElem := typedDestination.Elem()
		sourceSlice, sourceIsSlice := sourceUnderlying.(*types.Slice)
		sourceArray, sourceIsArray := sourceUnderlying.(*types.Array)

		if !sourceIsSlice && !sourceIsArray {
			return fmt.Errorf("unsupported mapping %s -> %s", g.renderType(source), g.renderType(destination))
		}

		if sourceIsSlice {
			writer.line("if in == nil {")
			writer.indent++
			writer.line("return nil")
			writer.indent--
			writer.line("}")
		}

		writer.linef("out := make(%s, len(in))", g.renderType(destination))
		writer.line("for i := range in {")
		writer.indent++

		var sourceElem types.Type
		if sourceIsSlice {
			sourceElem = sourceSlice.Elem()
		} else {
			sourceElem = sourceArray.Elem()
		}
		if types.AssignableTo(sourceElem, destinationElem) {
			writer.line("out[i] = in[i]")
		} else if g.isBasicConvertible(sourceElem, destinationElem) {
			writer.linef("out[i] = %s(in[i])", g.renderType(destinationElem))
		} else {
			elemHelper, err := g.ensureHelper(sourceElem, destinationElem)
			if err != nil {
				return err
			}
			writer.linef("out[i] = %s(in[i])", elemHelper)
		}

		writer.indent--
		writer.line("}")
		writer.line("return out")
		return nil
	case *types.Array:
		sourceArray, ok := sourceUnderlying.(*types.Array)
		if !ok {
			return fmt.Errorf("unsupported mapping %s -> %s", g.renderType(source), g.renderType(destination))
		}
		if sourceArray.Len() != typedDestination.Len() {
			return fmt.Errorf("unsupported mapping %s -> %s: array length mismatch", g.renderType(source), g.renderType(destination))
		}

		sourceElem := sourceArray.Elem()
		destinationElem := typedDestination.Elem()
		writer.linef("var out %s", g.renderType(destination))
		writer.line("for i := range in {")
		writer.indent++
		if types.AssignableTo(sourceElem, destinationElem) {
			writer.line("out[i] = in[i]")
		} else if g.isBasicConvertible(sourceElem, destinationElem) {
			writer.linef("out[i] = %s(in[i])", g.renderType(destinationElem))
		} else {
			elemHelper, err := g.ensureHelper(sourceElem, destinationElem)
			if err != nil {
				return err
			}
			writer.linef("out[i] = %s(in[i])", elemHelper)
		}
		writer.indent--
		writer.line("}")
		writer.line("return out")
		return nil
	case *types.Map:
		sourceMap, ok := sourceUnderlying.(*types.Map)
		if !ok {
			return fmt.Errorf("unsupported mapping %s -> %s", g.renderType(source), g.renderType(destination))
		}

		writer.line("if in == nil {")
		writer.indent++
		writer.line("return nil")
		writer.indent--
		writer.line("}")
		writer.linef("out := make(%s, len(in))", g.renderType(destination))

		keyTemp := g.nextTemp("sourceKey")
		valueTemp := g.nextTemp("sourceValue")
		writer.linef("for %s, %s := range in {", keyTemp, valueTemp)
		writer.indent++

		writer.linef("var mappedKey %s", g.renderType(typedDestination.Key()))
		if types.AssignableTo(sourceMap.Key(), typedDestination.Key()) {
			writer.linef("mappedKey = %s", keyTemp)
		} else if g.isBasicConvertible(sourceMap.Key(), typedDestination.Key()) {
			writer.linef("mappedKey = %s(%s)", g.renderType(typedDestination.Key()), keyTemp)
		} else {
			keyHelper, err := g.ensureHelper(sourceMap.Key(), typedDestination.Key())
			if err != nil {
				return err
			}
			writer.linef("mappedKey = %s(%s)", keyHelper, keyTemp)
		}

		writer.linef("var mappedValue %s", g.renderType(typedDestination.Elem()))
		if types.AssignableTo(sourceMap.Elem(), typedDestination.Elem()) {
			writer.linef("mappedValue = %s", valueTemp)
		} else if g.isBasicConvertible(sourceMap.Elem(), typedDestination.Elem()) {
			writer.linef("mappedValue = %s(%s)", g.renderType(typedDestination.Elem()), valueTemp)
		} else {
			valueHelper, err := g.ensureHelper(sourceMap.Elem(), typedDestination.Elem())
			if err != nil {
				return err
			}
			writer.linef("mappedValue = %s(%s)", valueHelper, valueTemp)
		}

		writer.line("out[mappedKey] = mappedValue")
		writer.indent--
		writer.line("}")
		writer.line("return out")
		return nil
	case *types.Interface:
		if typedDestination.NumMethods() == 0 {
			writer.line("return in")
			return nil
		}
	}

	if types.ConvertibleTo(source, destination) {
		writer.linef("return %s(in)", g.renderType(destination))
		return nil
	}

	return fmt.Errorf("unsupported mapping %s -> %s", g.renderType(source), g.renderType(destination))
}

func (g *codeGenerator) renderType(typ types.Type) string {
	return types.TypeString(typ, g.imports.qualifier)
}

func (g *codeGenerator) isBasicConvertible(source types.Type, destination types.Type) bool {
	sourceBasic, sourceIsBasic := types.Unalias(source).Underlying().(*types.Basic)
	destinationBasic, destinationIsBasic := types.Unalias(destination).Underlying().(*types.Basic)
	if !sourceIsBasic || !destinationIsBasic {
		return false
	}

	if sourceBasic.Info()&types.IsUntyped != 0 || destinationBasic.Info()&types.IsUntyped != 0 {
		return false
	}

	return types.ConvertibleTo(source, destination)
}

func (g *codeGenerator) nextTemp(prefix string) string {
	g.tempCounter++
	return fmt.Sprintf("%s%d", prefix, g.tempCounter)
}

func findExportedFieldByName(structType *types.Struct, name string) (*types.Var, bool) {
	for fieldIndex := 0; fieldIndex < structType.NumFields(); fieldIndex++ {
		field := structType.Field(fieldIndex)
		if field.Name() != name {
			continue
		}
		if !field.Exported() {
			return nil, false
		}
		return field, true
	}

	return nil, false
}

func parseTypeSpec(raw string) (parsedTypeSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsedTypeSpec{}, fmt.Errorf("empty type")
	}

	bracketDepth := 0
	for index := len(trimmed) - 1; index >= 0; index-- {
		switch trimmed[index] {
		case ']':
			bracketDepth++
		case '[':
			bracketDepth--
		case '.':
			if bracketDepth == 0 {
				packagePath := strings.TrimSpace(trimmed[:index])
				typeExpr := strings.TrimSpace(trimmed[index+1:])
				if packagePath == "" || typeExpr == "" {
					return parsedTypeSpec{}, fmt.Errorf("type must be in the form import/path.Type")
				}
				return parsedTypeSpec{PackagePath: packagePath, TypeExpr: typeExpr}, nil
			}
		}
	}

	return parsedTypeSpec{}, fmt.Errorf("type must be in the form import/path.Type")
}

func loadPackages(workingDirectory string, packagePaths []string) (map[string]*packages.Package, error) {
	config := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedSyntax |
			packages.NeedCompiledGoFiles |
			packages.NeedFiles,
		Dir: workingDirectory,
	}

	loaded, err := packages.Load(config, packagePaths...)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	byPath := map[string]*packages.Package{}
	for _, pkg := range loaded {
		if len(pkg.Errors) > 0 {
			messages := make([]string, 0, len(pkg.Errors))
			for _, packageError := range pkg.Errors {
				messages = append(messages, packageError.Msg)
			}
			return nil, fmt.Errorf("package %s has errors: %s", pkg.PkgPath, strings.Join(messages, "; "))
		}
		byPath[pkg.PkgPath] = pkg
	}

	for _, packagePath := range packagePaths {
		if _, exists := byPath[packagePath]; !exists {
			return nil, fmt.Errorf("package %s was not loaded", packagePath)
		}
	}

	return byPath, nil
}

func resolveType(loadedPackages map[string]*packages.Package, spec parsedTypeSpec) (resolvedType, error) {
	pkg, exists := loadedPackages[spec.PackagePath]
	if !exists {
		return resolvedType{}, fmt.Errorf("package %s not loaded", spec.PackagePath)
	}

	valueAndType, err := types.Eval(pkg.Fset, pkg.Types, token.NoPos, spec.TypeExpr)
	if err != nil {
		return resolvedType{}, fmt.Errorf("evaluate type expression %q in %s: %w", spec.TypeExpr, spec.PackagePath, err)
	}
	if valueAndType.Type == nil {
		return resolvedType{}, fmt.Errorf("expression %q did not resolve to a type", spec.TypeExpr)
	}
	if hasUnboundTypeParam(valueAndType.Type) {
		return resolvedType{}, fmt.Errorf("type %q has unbound type parameters; use a concrete instantiation", spec.TypeExpr)
	}

	return resolvedType{
		parsed: spec,
		typ:    valueAndType.Type,
	}, nil
}

func defaultFunctionName(sourceTypeExpression string, destinationTypeExpression string) string {
	sourceName := identifierFromTypeExpression(sourceTypeExpression)
	destinationName := identifierFromTypeExpression(destinationTypeExpression)
	name := sourceName + "To" + destinationName
	if !isValidIdentifier(name) {
		return "MapGenerated"
	}
	return name
}

func identifierFromTypeExpression(expression string) string {
	base := strings.TrimSpace(expression)
	if index := strings.Index(base, "["); index >= 0 {
		base = base[:index]
	}

	var builder strings.Builder
	capitalizeNext := true
	for _, runeValue := range base {
		if unicode.IsLetter(runeValue) || unicode.IsDigit(runeValue) {
			if capitalizeNext {
				builder.WriteRune(unicode.ToUpper(runeValue))
				capitalizeNext = false
			} else {
				builder.WriteRune(runeValue)
			}
			continue
		}
		capitalizeNext = true
	}

	result := builder.String()
	if result == "" {
		return "Type"
	}

	if first := []rune(result)[0]; !unicode.IsLetter(first) && first != '_' {
		return "T" + result
	}

	return result
}

func hasUnboundTypeParam(typ types.Type) bool {
	visited := map[types.Type]struct{}{}
	var walk func(types.Type) bool

	walk = func(current types.Type) bool {
		if current == nil {
			return false
		}
		if _, seen := visited[current]; seen {
			return false
		}
		visited[current] = struct{}{}

		switch typed := types.Unalias(current).(type) {
		case *types.TypeParam:
			return true
		case *types.Named:
			if typed.TypeParams().Len() > 0 && typed.TypeArgs().Len() == 0 {
				return true
			}
			for index := 0; index < typed.TypeArgs().Len(); index++ {
				if walk(typed.TypeArgs().At(index)) {
					return true
				}
			}
			return walk(typed.Underlying())
		case *types.Pointer:
			return walk(typed.Elem())
		case *types.Slice:
			return walk(typed.Elem())
		case *types.Array:
			return walk(typed.Elem())
		case *types.Map:
			return walk(typed.Key()) || walk(typed.Elem())
		case *types.Struct:
			for fieldIndex := 0; fieldIndex < typed.NumFields(); fieldIndex++ {
				if walk(typed.Field(fieldIndex).Type()) {
					return true
				}
			}
			return false
		case *types.Tuple:
			for index := 0; index < typed.Len(); index++ {
				if walk(typed.At(index).Type()) {
					return true
				}
			}
			return false
		case *types.Interface:
			for index := 0; index < typed.NumMethods(); index++ {
				if walk(typed.Method(index).Type()) {
					return true
				}
			}
			for index := 0; index < typed.NumEmbeddeds(); index++ {
				if walk(typed.EmbeddedType(index)) {
					return true
				}
			}
			return false
		case *types.Signature:
			if walk(typed.Params()) {
				return true
			}
			if walk(typed.Results()) {
				return true
			}
			if typed.TypeParams() != nil {
				for index := 0; index < typed.TypeParams().Len(); index++ {
					if walk(typed.TypeParams().At(index)) {
						return true
					}
				}
			}
			return false
		default:
			return false
		}
	}

	return walk(typ)
}

func isValidIdentifier(identifier string) bool {
	if identifier == "" {
		return false
	}

	runes := []rune(identifier)
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		return false
	}

	for _, runeValue := range runes[1:] {
		if !unicode.IsLetter(runeValue) && !unicode.IsDigit(runeValue) && runeValue != '_' {
			return false
		}
	}

	return true
}

func typePairKey(source types.Type, destination types.Type) string {
	return typeKey(source) + "->" + typeKey(destination)
}

func typeKey(typ types.Type) string {
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}

type importRegistry struct {
	packageName string
	packagePath string
	aliases     map[string]string
}

type importDeclaration struct {
	path  string
	alias string
}

func newImportRegistry(packageName string, packagePath string) *importRegistry {
	return &importRegistry{
		packageName: packageName,
		packagePath: packagePath,
		aliases:     map[string]string{},
	}
}

func (r *importRegistry) qualifier(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	if pkg.Path() == r.packagePath {
		return ""
	}
	if alias, exists := r.aliases[pkg.Path()]; exists {
		return alias
	}

	base := sanitizeIdentifier(path.Base(pkg.Path()))
	if base == "" {
		base = sanitizeIdentifier(pkg.Name())
	}
	if base == "" {
		base = "pkg"
	}
	if base == r.packageName {
		base += "pkg"
	}

	alias := base
	suffix := 2
	for r.aliasInUse(alias) {
		alias = fmt.Sprintf("%s%d", base, suffix)
		suffix++
	}

	r.aliases[pkg.Path()] = alias
	return alias
}

func (r *importRegistry) declarations() []importDeclaration {
	declarations := make([]importDeclaration, 0, len(r.aliases))
	for packagePath, alias := range r.aliases {
		declarations = append(declarations, importDeclaration{path: packagePath, alias: alias})
	}
	sort.Slice(declarations, func(left int, right int) bool {
		return declarations[left].path < declarations[right].path
	})
	return declarations
}

func (r *importRegistry) aliasInUse(alias string) bool {
	for _, existingAlias := range r.aliases {
		if existingAlias == alias {
			return true
		}
	}
	return false
}

func sanitizeIdentifier(raw string) string {
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	for _, runeValue := range raw {
		if unicode.IsLetter(runeValue) || unicode.IsDigit(runeValue) || runeValue == '_' {
			builder.WriteRune(runeValue)
		}
	}

	result := builder.String()
	if result == "" {
		return ""
	}

	runes := []rune(result)
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		result = "pkg" + result
	}

	return result
}

type codeWriter struct {
	builder strings.Builder
	indent  int
}

func (w *codeWriter) line(value string) {
	if value == "" {
		w.builder.WriteByte('\n')
		return
	}
	w.builder.WriteString(strings.Repeat("\t", w.indent))
	w.builder.WriteString(value)
	w.builder.WriteByte('\n')
}

func (w *codeWriter) linef(format string, args ...any) {
	w.line(fmt.Sprintf(format, args...))
}

func (w *codeWriter) String() string {
	return w.builder.String()
}

func (w *codeWriter) Len() int {
	return w.builder.Len()
}
