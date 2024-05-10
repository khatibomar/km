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
	"strings"
	"sync"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	configPath      = flag.String("config", "km.toml", "mapping configuration file")
	debug           = flag.Bool("debug", false, "log result instead of writing to files")
	routinesNumber  = flag.Int("routines", 1, "number of routines")
	errTypeNotFound = errors.New("specified type not found")
	errNoWork       = errors.New("no work to process")
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	var (
		cfg         Config
		batchWork   [][]work
		wg          sync.WaitGroup
		workErrChan = make(chan error, *routinesNumber)
		workChan    = make(chan []work, *routinesNumber)
	)

	cfgDir := filepath.Dir(*configPath)

	currDir, err := os.Getwd()
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	_, err = toml.DecodeFile(*configPath, &cfg)
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}

	style := cfg.Settings.Style
	switch style {
	case "standalone", "pointer", "value", "":
	default:
		log.Fatal().
			Msgf("Invalid style setting: %s", style)
	}

	if cfg.Settings.Module == "" {
		log.Fatal().
			Msg("module must be specified in config")
	}

	groupedMappings := groupMappings(cfg.Mappings)

	for _, mapping := range groupedMappings {
		var groupedWork []work
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

			groupedWork = append(groupedWork, work{
				Source:      source,
				Destination: destination,
			})
		}
		batchWork = append(batchWork, groupedWork)
	}

	results := make(chan File, len(batchWork))

	go handleWorkErrors(workErrChan)
	go worker(&wg, workChan, results, workErrChan, cfg)

	wg.Add(len(batchWork))
	for _, groupedWork := range batchWork {
		workChan <- groupedWork
	}

	close(workChan)
	close(workErrChan)
	wg.Wait()
	close(results)

	var generatedFiles []string

	for f := range results {
		if *debug {
			log.Print(string(f.Buf), "\n")
		} else {
			p := filepath.Join(currDir, cfgDir, f.Path, "km_gen.go")
			err = os.WriteFile(p, f.Buf, 0644)
			if err != nil {
				for _, p := range generatedFiles {
					removeErr := os.Remove(p)
					if removeErr != nil {
						log.Warn().
							Err(removeErr).
							Send()
					}
				}
				log.Fatal().
					Err(err).
					Send()
			}
			generatedFiles = append(generatedFiles, p)
		}
	}
}

func handleWorkErrors(errChan <-chan error) {
	for err := range errChan {
		log.Warn().
			Err(err).
			Send()
	}
}

func worker(wg *sync.WaitGroup, workChan <-chan []work, results chan<- File, errChan chan<- error, cfg Config) {
	for w := range workChan {
		defer wg.Done()
		g := Generator{
			style:          cfg.Settings.Style,
			module:         cfg.Settings.Module,
			pathFromModule: cfg.Settings.PathFromModule,
		}
		result, err := g.Process(w)
		if err != nil {
			errChan <- err
			return
		}
		results <- result
	}
}

func Usage() {
	log.Info().
		Msg("\nUsage of km:\nFlags:")
	flag.PrintDefaults()
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

func groupMappings(mappings []Mapping) map[string][]Mapping {
	groupedMappings := make(map[string][]Mapping)
	for _, m := range mappings {
		destinationPath := filepath.Dir(m.Destination.Path)
		groupedMappings[destinationPath] = append(groupedMappings[destinationPath], m)
	}

	return groupedMappings
}

type Generator struct {
	buf            bytes.Buffer
	style          string
	module         string
	pathFromModule string
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
			if sourceField.Type == destinationField.Type {
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
