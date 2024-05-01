package main

import (
	"flag"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	configPath = flag.String("config", "km.toml", "mapping configuration file")
	debug      = flag.Bool("debug", false, "enable debug logging")
)

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

	_, err = Run()
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}
}

func Run() (output []byte, err error) {
	return
}
