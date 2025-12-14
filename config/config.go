package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	BaseDir  string
	Filename string
	FullPath string
	Existing bool
	Error    error
	Debug    bool
}

func New() (*Config, error) {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)
	base_dir := exe_dir
	var candidates []string
	candidates = append(candidates, exe_dir)
	if _, caller_file, _, ok := runtime.Caller(1); ok {
		caller_dir := filepath.Dir(caller_file)
		candidates = append(candidates, caller_dir)
	}
	if _, this_file, _, ok2 := runtime.Caller(0); ok2 {
		cfg_dir := filepath.Dir(this_file)
		proj_root := filepath.Dir(cfg_dir)
		candidates = append(candidates, proj_root)
	}
	var config_filepath string
	var has_config bool
	for _, dir := range candidates {
		p := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(p); err == nil {
			base_dir = dir
			config_filepath = p
			has_config = true
			break
		}
	}
	filename := "config.yaml"
	if config_filepath == "" {
		config_filepath = filepath.Join(base_dir, filename)
	}
	viper.SetConfigFile(config_filepath)
	c := &Config{
		BaseDir:  base_dir,
		Filename: filename,
		FullPath: config_filepath,
		Existing: has_config,
	}
	return c, nil
}

func (c *Config) LoadConfig() error {
	if c.Existing {
		// config.FilePath = config_filepath
		if err := viper.ReadInConfig(); err != nil {
			var nf viper.ConfigFileNotFoundError
			if !(errors.As(err, &nf) || errors.Is(err, os.ErrNotExist)) {
				c.Error = err
				return err
			}
		}
	}
	return nil
}

// GetDebugInfo returns debug information about how the base directory was determined
func (c *Config) GetDebugInfo() map[string]string {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)

	info := map[string]string{
		"executable":    exe,
		"exe_dir":       exe_dir,
		"base_dir":      c.BaseDir,
		"config_path":   c.FullPath,
		"config_exists": fmt.Sprintf("%v", c.Existing),
	}

	// Determine run mode
	if filepath.Base(exe_dir) == "exe" || strings.Contains(exe, "go-build") {
		info["run_mode"] = "go run (development)"
	} else {
		info["run_mode"] = "compiled binary"
	}

	return info
}

func (c *Config) Update(key string, value interface{}) {
	viper.Set(key, value)
}

func (c *Config) Save() error {
	return viper.WriteConfigAs(c.FullPath)
}

func (c *Config) GetAll() map[string]interface{} {
	return viper.AllSettings()
}

func (c *Config) Get(key string) interface{} {
	return viper.Get(key)
}

// Typed getters with dotted path support, e.g. "a.b.c"
func (c *Config) GetString(path string) string   { return viper.GetString(path) }
func (c *Config) GetInt(path string) int         { return viper.GetInt(path) }
func (c *Config) GetBool(path string) bool       { return viper.GetBool(path) }
func (c *Config) GetFloat64(path string) float64 { return viper.GetFloat64(path) }

func EnsureDirIfMissing(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return err
}
