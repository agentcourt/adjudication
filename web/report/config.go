package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const DefaultListen = "127.0.0.1:19980"

// Root is one directory tree scanned for run output directories.  Name
// appears in URLs and page text.
type Root struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Config struct {
	Listen string `json:"listen"`
	Roots  []Root `json:"roots"`
}

var rootNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// LoadConfig builds the server configuration from an optional JSON
// config file, roots given on the command line, and a listen address
// given on the command line.  Command-line roots follow config-file
// roots.  A non-empty listen argument overrides the config file.
func LoadConfig(configPath string, flagRoots []Root, listen string) (Config, error) {
	var cfg Config
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", configPath, err)
		}
	}
	cfg.Roots = append(cfg.Roots, flagRoots...)
	if listen != "" {
		cfg.Listen = listen
	}
	if cfg.Listen == "" {
		cfg.Listen = DefaultListen
	}
	if len(cfg.Roots) == 0 {
		return Config{}, fmt.Errorf("no roots configured")
	}
	if err := finishRoots(cfg.Roots); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ParseRootArg parses a --root argument of the form path or name=path.
func ParseRootArg(arg string) (Root, error) {
	name, path := "", arg
	if i := strings.Index(arg, "="); i >= 0 {
		name, path = arg[:i], arg[i+1:]
	}
	if path == "" {
		return Root{}, fmt.Errorf("root %q has an empty path", arg)
	}
	return Root{Name: name, Path: path}, nil
}

// finishRoots makes paths absolute, verifies each root is a readable
// directory, fills empty names from path base names, and requires the
// final names to be unique and URL-safe.
func finishRoots(roots []Root) error {
	used := map[string]bool{}
	usedPaths := map[string]bool{}
	for i := range roots {
		abs, err := filepath.Abs(roots[i].Path)
		if err != nil {
			return fmt.Errorf("root %s: %w", roots[i].Path, err)
		}
		if usedPaths[abs] {
			return fmt.Errorf("root path %s is configured twice", abs)
		}
		usedPaths[abs] = true
		roots[i].Path = abs
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("root %s: %w", abs, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("root %s is not a directory", abs)
		}
		name := roots[i].Name
		if name == "" {
			name = filepath.Base(abs)
		}
		if !rootNamePattern.MatchString(name) {
			return fmt.Errorf("root name %q must match %s", name, rootNamePattern)
		}
		base := name
		for n := 2; used[name]; n++ {
			name = fmt.Sprintf("%s-%d", base, n)
		}
		used[name] = true
		roots[i].Name = name
	}
	return nil
}
