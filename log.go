package main

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	stdLogger *log.Logger
}

var defaultLogger = New(os.Stderr)

func New(out *os.File) *Logger {
	return &Logger{
		stdLogger: log.New(out, "", log.LstdFlags),
	}
}

func (l *Logger) Printf(format string, args ...any) {
	l.stdLogger.Printf(format+"\n", args...)
}

func (l *Logger) Info(format string, args ...any) {
	l.stdLogger.Printf("INFO: "+format+"\n", args...)
}

func (l *Logger) Warning(format string, args ...any) {
	l.stdLogger.Printf("WARNING: "+format+"\n", args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.stdLogger.Printf("ERROR: "+format+"\n", args...)
}

func (l *Logger) Debug(format string, args ...any) {
	l.stdLogger.Printf("DEBUG: "+format+"\n", args...)
}

func (l *Logger) Fatal(format string, args ...any) {
	l.stdLogger.Printf("CRITICAL: "+format+"\n", args...)
	panic(fmt.Sprintf(format+"\n", args...))
}
