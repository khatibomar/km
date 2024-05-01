package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	configPath = flag.String("config", "km.toml", "mapping configuration file")
	debug      = flag.Bool("debug", false, "enable debug logging")
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	var config Config
	_, err := toml.DecodeFile(*configPath, &config)
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}

	result, err := Run(config)
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}

	for _, r := range result {
		log.Print(string(r.Buf))
	}
}

func Usage() {
	log.Info().
		Msg("\nUsage of km:\nFlags:")
	flag.PrintDefaults()
}

func Run(cfg Config) ([]File, error) {
	cfgDir := filepath.Dir(*configPath)
	groupedMappings := make(map[string][]Mapping)
	for _, m := range cfg.Mappings {
		destinationPath := filepath.Dir(m.Destination.Path)
		groupedMappings[destinationPath] = append(groupedMappings[destinationPath], m)
	}

	var result []File

	for key, mapping := range groupedMappings {
		g := Generator{}

		for i, m := range mapping {
			sourceNode, err := loadAstFromFile(path.Join(cfgDir, m.Source.Path))
			if err != nil {
				return result, err
			}
			destinationNode, err := loadAstFromFile(path.Join(cfgDir, m.Destination.Path))
			if err != nil {
				return result, err
			}

			if i == 0 {
				g.Printf("package %s\n", getPackage(destinationNode))
			}

			ignoredMap := make(map[string]struct{})
			for _, ignoreField := range m.Destination.IgnoredFields {
				ignoredMap[ignoreField] = struct{}{}
			}

			source := SourceData{
				name: m.Source.Name,
				node: sourceNode,
				path: m.Source.Path,
			}

			destination := DestinationData{
				name:       m.Destination.Name,
				node:       destinationNode,
				path:       m.Destination.Path,
				ignoredMap: ignoredMap,
				fieldsMap:  m.Destination.FieldsMap,
			}

			if err := g.generate(source, destination); err != nil {
				return result, err
			}
		}

		result = append(result, File{
			Path: key,
			Buf:  g.format(),
		})
	}

	return result, nil
}

type Generator struct {
	buf bytes.Buffer
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

func (g *Generator) generate(source SourceData, destination DestinationData) error {
	sourceFields := getFields(source.node, source.name)
	sourceFieldsLookup := make(map[string]Field)
	for _, f := range sourceFields {
		sourceFieldsLookup[f.Name] = f
	}
	destinationFields := getFields(destination.node, destination.name)

	var destinationName string

	samePkg := filepath.Dir(source.path) == filepath.Dir(destination.path)

	if samePkg {
		destinationName = destination.name
	} else {
		destinationName = fmt.Sprintf("%s.%s", getPackage(destination.node), destination.name)
	}

	g.Printf("func (dest *%s) From%s(src %s) {", source.name, destination.name, destinationName)

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
			if sourceField.Type == destinationField.Type {
				g.Printf("dest.%s = src.%s\n", sourceField.Name, destinationField.Name)
			}
		}
	}

	g.Printf("}")

	return nil
}

func loadAstFromFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, path, nil, 0)
}

func getPackage(node *ast.File) string {
	return node.Name.String()
}

func getFields(node *ast.File, targetStructName string) []Field {
	var fields []Field

	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.String() == targetStructName {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					for _, f := range structType.Fields.List {
						for _, n := range f.Names {
							fields = append(fields, Field{
								Name: n.Name,
								Type: fmt.Sprintf("%s", f.Type),
							})
						}
					}
				}
				return false
			}
		}
		return true
	})

	return fields
}

type Field struct {
	Name string
	Type string
}

type File struct {
	Path string
	Buf  []byte
}

type SourceData struct {
	node *ast.File
	path string
	name string
}

type DestinationData struct {
	node       *ast.File
	path       string
	name       string
	ignoredMap map[string]struct{}
	fieldsMap  map[string]string
}
