package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	debug = flag.Bool("debug", false, "enable debug logging")
)

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

	_, err := Run()
	if err != nil {
		log.Fatal().
			Err(err).
			Send()
	}
}

func Run() (output []byte, err error) {
	return
}
