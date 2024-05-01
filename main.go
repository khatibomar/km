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

	_, err = Run(config)
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}
}

type Config struct {
	Mappings []struct {
		Settings struct {
			Override bool `toml:"override"`
		} `toml:"settings"`
		Source struct {
			Name string `toml:"name"`
			Path string `toml:"path"`
		} `toml:"source"`
		Destination struct {
			Name          string            `toml:"name"`
			Path          string            `toml:"path"`
			IgnoredFields []string          `toml:"ignore"`
			FieldsMap     map[string]string `toml:"map"`
		} `toml:"destination"`
	} `toml:"mappings"`
}

func Usage() {
	log.Info().
		Msg("\nUsage of km:\nFlags:")
	flag.PrintDefaults()
}

func Run(cfg Config) ([]byte, error) {
	g := Generator{}

	g.generate()

	return g.format(), nil
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

func (g *Generator) generate() {}

func loadAstFromFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, path, nil, 0)
}

func getFields(node *ast.File, targetStructName string) (string, []Field) {
	pkg := node.Name.String()
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

	return pkg, fields
}

type Field struct {
	Name string
	Type string
}
