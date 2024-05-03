package main

import (
	"bytes"
	"errors"
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
	configPath      = flag.String("config", "km.toml", "mapping configuration file")
	debug           = flag.Bool("debug", false, "enable debug logging")
	errTypeNotFound = errors.New("specified type not found")
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	cfgDir := filepath.Dir(*configPath)

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	var cfg Config
	_, err := toml.DecodeFile(*configPath, &cfg)
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}

	groupedMappings := groupMappings(cfg.Mappings)

	var w []work

	for _, mapping := range groupedMappings {
		for _, m := range mapping {
			sourceNode, err := loadAstFromFile(path.Join(cfgDir, m.Source.Path))
			if err != nil {
				log.Fatal().
					Err(err).
					Send()
			}
			destinationNode, err := loadAstFromFile(path.Join(cfgDir, m.Destination.Path))
			if err != nil {
				log.Fatal().
					Err(err).
					Send()
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

			w = append(w, work{
				Source:      source,
				Destination: destination,
			})
		}
	}

	result, err := Process(w)
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

func Process(work []work) ([]File, error) {
	var result []File

	for i, w := range work {

		g := Generator{}

		if i == 0 {
			g.Printf("package %s\n", getPackage(w.Destination.node))
		}

		if err := g.generate(w.Source, w.Destination); err != nil {
			return result, err
		}

		result = append(result, File{
			Path: w.Destination.path,
			Buf:  g.format(),
		})
	}

	return result, nil
}

func groupMappings(mappings []Mapping) map[string][]Mapping {
	groupedMappings := make(map[string][]Mapping)
	for _, m := range mappings {
		destinationPath := filepath.Dir(m.Destination.Path)
		groupedMappings[destinationPath] = append(groupedMappings[destinationPath], m)
	}

	return groupedMappings
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
				g.Printf("dest.%s = src.%s\n", destinationField.Name, sourceField.Name)
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

func getFields(node *ast.File, typeName string) ([]Field, error) {
	var fields []Field
	var found bool
	var err error

	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.String() == typeName {
				found = true
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

	if !found {
		err = fmt.Errorf("type(%s): %w", typeName, errTypeNotFound)
	}

	return fields, err
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

type work struct {
	Source      SourceData
	Destination DestinationData
}
