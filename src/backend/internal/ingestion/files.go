package ingestion

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Source file names inside the input directory.
const (
	ReadingsFile = "readings.csv"
	EventsFile   = "events.csv"
)

// ParseDir parses and validates both source files in dir. Nothing is written
// anywhere until both files are fully valid.
func ParseDir(dir string) (Dataset, error) {
	readings, err := parseFile(filepath.Join(dir, ReadingsFile), ParseReadings)
	if err != nil {
		return Dataset{}, err
	}
	events, err := parseFile(filepath.Join(dir, EventsFile), ParseEvents)
	if err != nil {
		return Dataset{}, err
	}
	return Dataset{Readings: readings, Events: events}, nil
}

func parseFile[T any](path string, parse func(io.Reader) ([]T, error)) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer f.Close()
	return parse(f)
}
