package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Record is a normalized usage record emitted by any agent parser.
type Record struct {
	Agent            string
	CacheReadTokens  int64
	CacheWriteTokens int64
	InputTokens      int64
	Model            string
	OutputTokens     int64
	Project          string
	ReasoningTokens  int64
	Timestamp        time.Time
}

// Parser is the interface each agent implements.
type Parser interface {
	Name() string
	Discover() ([]string, error)
	Parse(path string) ([]Record, error)
}

var (
	mu       sync.Mutex
	registry = map[string]Parser{}
)

// Register adds a parser to the global registry.
func Register(p Parser) {
	mu.Lock()
	registry[p.Name()] = p
	mu.Unlock()
}

// All returns all registered parsers in alphabetical order by name.
func All() []Parser {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sortStrings(names)
	parsers := make([]Parser, 0, len(names))
	for _, n := range names {
		parsers = append(parsers, registry[n])
	}
	return parsers
}

// Get returns a parser by name, or an error if not found.
func Get(name string) (Parser, error) {
	mu.Lock()
	defer mu.Unlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", name)
	}
	return p, nil
}

// discover globs the tail pattern under each root and returns every matching file.
func discover(roots []string, tail string) ([]string, error) {
	var paths []string
	for _, root := range roots {
		matches, err := filepath.Glob(filepath.Join(root, tail))
		if err != nil {
			continue
		}
		paths = append(paths, matches...)
	}
	return paths, nil
}

// rootsFromEnv returns the OS-list-separated directories in the named environment
// variable, or fallback when the variable is unset or empty. This lets a user point
// any parser at one or more custom log locations without touching the code.
func rootsFromEnv(name string, fallback []string) []string {
	if v := os.Getenv(name); v != "" {
		return filepath.SplitList(v)
	}
	return fallback
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
