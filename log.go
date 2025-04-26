package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarningLevel
	ErrorLevel
	FatalLevel
)

type Logger struct {
	stdLogger *log.Logger
	errLogger *log.Logger
	level     LogLevel
	out       io.Writer
	errorOut  io.Writer
}

var defaultLogger = NewLogger(os.Stdout, os.Stderr, InfoLevel)

func NewLogger(out *os.File, errOut *os.File, defaultLevel LogLevel) *Logger {
	return &Logger{
		stdLogger: log.New(out, "", log.LstdFlags),
		errLogger: log.New(errOut, "", log.LstdFlags),
		level:     defaultLevel,
		out:       out,
		errorOut:  os.Stderr,
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) Printf(format string, args ...any) {
	l.stdLogger.Printf(format+"\n", args...)
}

func (l *Logger) Info(format string, args ...any) {
	if l.level <= InfoLevel {
		l.stdLogger.Printf("INFO: "+format+"\n", args...)
	}
}

func (l *Logger) Warning(format string, args ...any) {
	if l.level <= WarningLevel {
		l.errLogger.Printf("WARNING: "+format+"\n", args...)
	}
}

func (l *Logger) Error(format string, args ...any) {
	if l.level <= ErrorLevel {
		l.errLogger.Printf("ERROR: "+format+"\n", args...)
	}
}

func (l *Logger) Debug(format string, args ...any) {
	if l.level <= DebugLevel {
		l.stdLogger.Printf("DEBUG: "+format+"\n", args...)
	}
}

func (l *Logger) Fatal(format string, args ...any) {
	l.errLogger.Printf("CRITICAL: "+format+"\n", args...)
	panic(fmt.Sprintf(format+"\n", args...))
}
