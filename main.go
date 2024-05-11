package main

import (
	"errors"
	"flag"
	"go/ast"
	"os"
	"path"
	"path/filepath"
	"sync"

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

func groupMappings(mappings []Mapping) map[string][]Mapping {
	groupedMappings := make(map[string][]Mapping)
	for _, m := range mappings {
		destinationPath := filepath.Dir(m.Destination.Path)
		groupedMappings[destinationPath] = append(groupedMappings[destinationPath], m)
	}

	return groupedMappings
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
