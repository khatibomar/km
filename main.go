package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/BurntSushi/toml"
)

const logo = `
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣶⣶⠒⠖⢰⣶⠶⢶⡶⠀⣶⡶⠶⠆⢰⣶⠶⠶⠂⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⣿⣴⡄⢹⣿⣦⣾⣋⠐⣿⣷⣤⡄⣸⣿⣤⣤⠀⠀⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⣿⠀⠀⢹⣿⠀⢹⣯⠀⣿⣧⢀⡀⢼⣿⡀⣀⠀⠀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠂⠠⠀⠒⠐⠒⠀⠀⠀⠛⠛⠀⠀⠘⠛⠀⠘⠓⠈⠛⠛⠛⠃⠘⠛⠛⠛⠀⠀⠑⠂⠀⠐⠂⠀⠀⠀⠀⠀⠒⠀⠀
⠀⠀⣀⣀⣀⣀⠀⠀⢀⣀⣀⠀⠀⣀⣀⠀⠀⢀⣀⣀⣀⠀⣀⣀⣀⡀⢀⣀⣀⣀⣀⢀⣀⡀⢀⣀⡀⠀⣀⡀⢀⣀⣀⣀⡀⠀⠃
⠀⠀⣿⡿⠙⣿⡇⠀⣿⡟⣿⡄⠀⣿⣇⠀⠀⢸⣿⠋⠛⢸⣿⡋⠻⠷⠈⢛⣿⡟⠙⢸⣿⡇⢸⣿⣷⠀⣿⡇⢸⣿⡟⠙⠃⠀⡆
⠀⠀⣿⣿⣶⣿⠇⢰⣿⣇⣿⣇⠀⣿⡧⠀⠀⢸⣿⠶⠶⠀⠛⢿⣶⣄⠀⢨⣿⡇⠀⢸⣿⡇⢸⣿⢻⣇⣿⡇⢸⣿⡷⠶⠀⠀⠃
⠀⠀⣿⣷⠀⠀⠀⣼⣿⠛⢻⣿⠀⣿⣿⣠⡀⢼⣿⣄⣄⢼⣾⣄⣽⣿⠂⠰⣿⡇⠀⢸⣿⡇⢸⣿⠀⢿⣿⡇⢸⣿⣧⣠⡄⠀⠀
⠀⠀⠉⠁⠀⠀⠀⠉⠁⠀⠈⠉⠁⠉⠉⠉⠀⠈⠉⠉⠉⠀⠉⠉⠉⠁⠀⠀⠉⠁⠀⠀⠉⠁⠀⠉⠀⠈⠉⠁⠈⠉⠉⠉⠁⠀⠀
⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀
⠀⠀⣿⡽⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀
⠀⠀⣯⣟⣷⣻⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀
⠀⠀⣿⣞⣷⣻⢯⣟⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀
⠀⠀⣷⣻⢾⡽⣯⣟⣾⡽⣿⣿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠿⠇⠀⠀
⠀⠀⣿⡽⣯⣟⡷⣯⡷⣿⡽⣯⣦⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⣟⡷⣯⣟⡷⣯⡷⣯⢷⣯⣟⡷⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⣿⡽⣷⢯⣟⡷⣿⣽⣻⢾⣽⣻⣽⠗⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⣯⢿⡽⣯⢿⣽⣳⣯⣟⡿⣾⠝⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⣿⢯⡿⣽⣻⢾⣽⢾⣽⣟⡁⣀⣀⣀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⢀⡀⣀⡀⠀⠀
⠀⠀⡿⣯⢿⡽⣯⢿⣞⣿⢻⠼⣱⢣⢞⡲⡝⣮⢳⡝⣮⢳⡝⣮⢳⡝⣮⢳⡝⣮⢳⡝⣮⢳⡝⣮⢳⡝⣮⢳⡝⣎⢷⣣⡇⠀⠀
⠀⠀⣿⡽⣯⢿⣽⡿⣏⣳⢫⣝⡱⣏⠾⣱⢏⢶⢫⡜⡶⢫⡜⡶⢫⡜⡶⢫⡜⡶⢫⡜⡶⢫⡜⡶⢫⡜⡶⢫⡼⢭⡖⣧⡇⠀⠀
⠀⠀⣿⡽⣯⡿⣏⡳⠵⣎⡳⢎⡗⣮⢛⡵⣎⠯⣞⡹⣜⢧⣛⡵⣫⢞⡹⣇⢯⣓⠯⣞⡹⣇⢯⣓⠯⣞⡹⢧⡝⡶⣹⠖⡇⠀⠀
⠀⠀⣯⣿⢻⡱⢇⣻⣙⢶⣹⢫⡞⣵⢫⡞⣭⢻⣜⡳⣝⢮⢧⡽⣱⢫⣳⡹⣎⣭⢻⡜⣧⢻⡼⣩⢟⡼⣭⢳⡝⡞⣵⢫⡇⠀⢰
⡀⠀⠛⠃⠓⠋⠛⠒⠙⠚⠒⠋⠚⠘⠃⠛⠘⠓⠊⠓⠊⠓⠚⠒⠉⠓⠃⠓⠙⠒⠋⠚⠘⠓⠚⠑⠋⠒⠓⠋⠚⠙⠒⠛⠃⠀⡘
⡰⠀⠀⢀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣶⠄⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣤⠀⣠⡀⠀⠈⠀
⠀⠀⢻⠚⠋⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⠀⠀⠀⠀⠀⠀⢸⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⠀⢨⡄⠀⢸⠀
⠁⠀⠀⣦⣄⠀⠀⠀⠀⠀⣴⡀⢠⣴⣤⣀⠀⠀⠀⠀⠀⠉⠀⣀⠀⠠⣄⠀⠀⠇⣀⣤⣄⠀⢀⠀⠀⡀⣀⠀⢸⠀⣏⣻⠀⠀⠆
⡆⠀⢰⣅⣹⠇⠀⠀⠀⠀⠈⣧⣤⣤⣬⣽⣷⣤⠀⡌⠀⠀⠀⠹⣦⣤⣥⣤⣼⣯⣥⣤⣿⣤⣼⣦⣤⣷⣼⣤⣼⣦⣭⣽⡃⠀⠐
⠡⡀⠀⠉⠁⠀⠀⠀⠀⢀⣴⠃⠁⠈⠀⠁⠀⠁⠠⣧⣀⣀⣠⡴⢃⣩⡤⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠂
⠀⠀⠁⠐⠄⠀⠉⠒⠚⠋⠁⠀⠔⠀⠀⠀⠀⠀⡀⠈⠉⠉⠁⠀⠈⠀⠀⠀⠀⠉⠉⠉⠀⠉⠁⠉⠉⠁⠀⠉⠈⠁⠀⠀⠉⠀⠀
⠀⠀⠀⠀⠀⠐⠀⠤⠤⠄⠀⠁⠀⠀⠀⠀⠀⠀⠈⠀⠒⠂⠀⠉⠁⠀⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
`

var (
	configPath      = flag.String("config", "km.toml", "mapping configuration file")
	debugging       = flag.Bool("debug", false, "log result instead of writing to files")
	routinesNumber  = flag.Int("routines", 1, "number of routines")
	errTypeNotFound = errors.New("specified type not found")
	errNoWork       = errors.New("no work to process")
)

var version = "dev"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
	}
}

func main() {
	fmt.Print(logo)

	if *debugging {
		defaultLogger = NewLogger(os.Stdout, os.Stderr, DebugLevel)
	}

	defaultLogger.Info("Version: %s", version)

	flag.Usage = Usage
	flag.Parse()

	var (
		cfg         Config
		batchWork   [][]workTyper
		wg          sync.WaitGroup
		workErrChan = make(chan error, *routinesNumber)
		workChan    = make(chan []workTyper, *routinesNumber)
	)

	schema := cfg.BuildTOMLSchema()
	schemaPath := filepath.Join(filepath.Dir(*configPath), ".km_schema.json")
	defaultLogger.Info("Generating TOML schema file: %s", schemaPath)
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		defaultLogger.Fatal("Error writing schema file: %v", err)
	}

	cfgDir := filepath.Dir(*configPath)

	currDir, err := os.Getwd()
	if err != nil {
		defaultLogger.Fatal("%v", err)
	}

	_, err = toml.DecodeFile(*configPath, &cfg)
	if err != nil {
		defaultLogger.Fatal("%v", err)
	}

	style := cfg.Settings.Style
	switch style {
	case "standalone", "pointer", "value", "":
	default:
		defaultLogger.Fatal("Invalid style setting: %s", style)
	}

	if cfg.Settings.Module == "" {
		defaultLogger.Fatal("Module must be specified in config")
	}

	groupedMappings := groupMappings(cfg.Mappings)

	for _, mapping := range groupedMappings {
		var groupedWork []workTyper
		for _, m := range mapping {
			sourceNode, err := loadAstFromFile(path.Join(cfgDir, m.Source.Path))
			if err != nil {
				defaultLogger.Fatal("Error loading source file: %v", err)
			}

			for _, d := range m.Destinations {
				destinationNode, err := loadAstFromFile(path.Join(cfgDir, d.Path))
				if err != nil {
					defaultLogger.Fatal("Error loading destination file: %v", err)
				}

				ignoredMap := make(map[string]struct{})
				for _, ignoreField := range d.IgnoredFields {
					ignoredMap[ignoreField] = struct{}{}
				}

				source := SourceData{
					name: m.Source.Name,
					node: sourceNode,
					path: m.Source.Path,
				}

				destination := DestinationData{
					name:       d.Name,
					node:       destinationNode,
					path:       d.Path,
					ignoredMap: ignoredMap,
					fieldsMap:  d.FieldsMap,
				}

				groupedWork = append(groupedWork, work{
					Source:      source,
					Destination: destination,
				})
			}
		}
		if len(groupedWork) == 0 {
			continue
		}
		batchWork = append(batchWork, groupedWork)
	}

	mapPluginWork := make([]workTyper, 0)
	for _, m := range cfg.Mappings {
		if m.Source.Plugins == nil {
			continue
		}
		for _, plugin := range m.Source.Plugins {
			sourceNode, err := loadAstFromFile(path.Join(cfgDir, m.Source.Path))
			if err != nil {
				defaultLogger.Fatal("Error loading source file: %v", err)
			}
			mapPluginWork = append(mapPluginWork, mapWork{
				Target: SourceData{
					name: m.Source.Name,
					node: sourceNode,
					path: m.Source.Path,
				},
				plugin: getMapType(plugin),
			})
		}
	}

	results := make(chan File, len(batchWork)+len(mapPluginWork))

	go handleWorkErrors(workErrChan)

	numWorkers := *routinesNumber
	for range numWorkers {
		go worker(&wg, workChan, results, workErrChan, cfg)
	}

	if len(mapPluginWork) > 0 {
		wg.Add(1)
		workChan <- mapPluginWork
	}

	for _, groupedWork := range batchWork {
		wg.Add(1)
		workChan <- groupedWork
	}

	close(workChan)
	close(workErrChan)
	wg.Wait()
	close(results)

	var generatedFiles []string

	for f := range results {
		if *debugging {
			defaultLogger.Debug("%s", string(f.Buf))
		} else {
			p := filepath.Join(currDir, cfgDir, f.Path, "km_gen.go")
			err = os.WriteFile(p, f.Buf, 0644)
			if err != nil {
				for _, p := range generatedFiles {
					removeErr := os.Remove(p)
					if removeErr != nil {
						defaultLogger.Warning("%v", removeErr)
					}
				}
				defaultLogger.Fatal("Error writing file: %v", err)
			}
			generatedFiles = append(generatedFiles, p)
			defaultLogger.Info("Generated file %s", p)
		}
	}
	defaultLogger.Info("Generated %d files", len(generatedFiles))
}

func handleWorkErrors(errChan <-chan error) {
	for err := range errChan {
		defaultLogger.Warning("%v", err)
	}
}

func worker(wg *sync.WaitGroup, workChan <-chan []workTyper, results chan<- File, errChan chan<- error, cfg Config) {
	for w := range workChan {
		defer wg.Done()
		g := &Generator{
			style:          cfg.Settings.Style,
			module:         cfg.Settings.Module,
			pathFromModule: cfg.Settings.PathFromModule,
		}
		result, err := Process(g, w)
		if err != nil {
			errChan <- err
			return
		}
		results <- result
	}
}

func Usage() {
	defaultLogger.Printf("\nUsage of km:\nFlags:")
	flag.PrintDefaults()
}

func groupMappings(mappings []Mapping) map[string][]Mapping {
	groupedMappings := make(map[string][]Mapping)
	for _, m := range mappings {
		for _, d := range m.Destinations {
			destinationPath := filepath.Dir(d.Path)
			groupedMappings[destinationPath] = append(groupedMappings[destinationPath], m)
		}
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

func (w work) isWork() {}

type MapPlugin int

const (
	FromMap MapPlugin = iota
	ToMap
)

func getMapType(plugin string) MapPlugin {
	switch plugin {
	case "from_map":
		return FromMap
	case "to_map":
		return ToMap
	default:
		panic(fmt.Sprintf("unsupported plugin: %s", plugin))
	}
}

type mapWork struct {
	Target SourceData
	plugin MapPlugin
}

func (mw mapWork) isWork() {}

type workTyper interface {
	isWork()
}
