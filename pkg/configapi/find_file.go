package configapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindConfigFileOptions describes how a configuration file should be located.
// An explicit path, including one supplied through EnvironmentVariable, wins
// over SearchDirectories even when the file does not exist yet.
type FindConfigFileOptions struct {
	ExplicitPath        string
	EnvironmentVariable string
	Filename            string
	SearchDirectories   []string
	FallbackDirectory   string
}

// ConfigFileLocation is the resolved location of a configuration file.
type ConfigFileLocation struct {
	Path      string
	Directory string
	Filename  string
	Exists    bool
	Explicit  bool
}

// FindConfigFile resolves an explicit configuration path or finds the first
// existing file in SearchDirectories. When no file exists, it returns the
// default path under FallbackDirectory so writable sources have a stable target.
func FindConfigFile(options FindConfigFileOptions) (ConfigFileLocation, error) {
	explicit_path := strings.TrimSpace(options.ExplicitPath)
	if explicit_path == "" && strings.TrimSpace(options.EnvironmentVariable) != "" {
		explicit_path = strings.TrimSpace(os.Getenv(options.EnvironmentVariable))
	}
	if explicit_path != "" {
		path, err := filepath.Abs(explicit_path)
		if err != nil {
			return ConfigFileLocation{}, fmt.Errorf("configapi: resolve explicit config file: %w", err)
		}
		return config_file_location(path, true), nil
	}

	filename := strings.TrimSpace(options.Filename)
	if filename == "" {
		filename = "config.yaml"
	}
	for _, directory := range options.SearchDirectories {
		directory = strings.TrimSpace(directory)
		if directory == "" {
			continue
		}
		path, err := filepath.Abs(filepath.Join(directory, filename))
		if err != nil {
			return ConfigFileLocation{}, fmt.Errorf("configapi: resolve config search path: %w", err)
		}
		location := config_file_location(path, false)
		if location.Exists {
			return location, nil
		}
	}

	fallback_directory := strings.TrimSpace(options.FallbackDirectory)
	if fallback_directory == "" {
		for _, directory := range options.SearchDirectories {
			if strings.TrimSpace(directory) != "" {
				fallback_directory = directory
				break
			}
		}
	}
	if fallback_directory == "" {
		fallback_directory = "."
	}
	path, err := filepath.Abs(filepath.Join(fallback_directory, filename))
	if err != nil {
		return ConfigFileLocation{}, fmt.Errorf("configapi: resolve default config file: %w", err)
	}
	return config_file_location(path, false), nil
}

func config_file_location(path string, explicit bool) ConfigFileLocation {
	path = filepath.Clean(path)
	exists := false
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		exists = true
	}
	return ConfigFileLocation{
		Path:      path,
		Directory: filepath.Dir(path),
		Filename:  filepath.Base(path),
		Exists:    exists,
		Explicit:  explicit,
	}
}
