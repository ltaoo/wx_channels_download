package configapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type FileFormat string

const (
	FormatAuto FileFormat = "auto"
	FormatYAML FileFormat = "yaml"
	FormatJSON FileFormat = "json"
)

type FileSourceOptions struct {
	Name     string
	Path     string
	Priority int
	Format   FileFormat
	Optional bool
	Writable bool
	Mode     os.FileMode
}

type FileSource struct {
	options FileSourceOptions
}

func NewFileSource(options FileSourceOptions) (*FileSource, error) {
	options.Name = normalize_source_name(options.Name)
	options.Path = filepath.Clean(strings.TrimSpace(options.Path))
	if options.Name == "" {
		return nil, errors.New("configapi: file source name is empty")
	}
	if options.Path == "" || options.Path == "." {
		return nil, errors.New("configapi: file source path is empty")
	}
	if options.Format == "" {
		options.Format = FormatAuto
	}
	if options.Mode == 0 {
		options.Mode = 0600
	}
	if _, err := resolve_file_format(options.Path, options.Format); err != nil {
		return nil, err
	}
	return &FileSource{options: options}, nil
}

func (s *FileSource) Name() string { return s.options.Name }

func (s *FileSource) Priority() int { return s.options.Priority }

func (s *FileSource) Path() string { return s.options.Path }

func (s *FileSource) Writable() bool { return s != nil && s.options.Writable }

func (s *FileSource) Load(context.Context) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("configapi: file source is nil")
	}
	data, err := os.ReadFile(s.options.Path)
	if err != nil {
		if s.options.Optional && errors.Is(err, os.ErrNotExist) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return make(map[string]any), nil
	}
	format, _ := resolve_file_format(s.options.Path, s.options.Format)
	values := make(map[string]any)
	switch format {
	case FormatJSON:
		err = json.Unmarshal(data, &values)
	case FormatYAML:
		err = yaml.Unmarshal(data, &values)
	}
	if err != nil {
		return nil, fmt.Errorf("configapi: decode %s: %w", s.options.Path, err)
	}
	return clone_values(values)
}

func (s *FileSource) Store(_ context.Context, values map[string]any) error {
	if s == nil {
		return errors.New("configapi: file source is nil")
	}
	if !s.options.Writable {
		return fmt.Errorf("configapi: source %s is read-only", s.options.Name)
	}
	cloned, err := clone_values(values)
	if err != nil {
		return err
	}
	format, _ := resolve_file_format(s.options.Path, s.options.Format)
	var data []byte
	switch format {
	case FormatJSON:
		data, err = json.MarshalIndent(cloned, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
	case FormatYAML:
		data, err = yaml.Marshal(cloned)
	}
	if err != nil {
		return fmt.Errorf("configapi: encode %s: %w", s.options.Path, err)
	}
	dir := filepath.Dir(s.options.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".configapi-*")
	if err != nil {
		return err
	}
	temporary_path := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporary_path)
		}
	}()
	if err := temporary.Chmod(s.options.Mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary_path, s.options.Path); err != nil {
		return err
	}
	committed = true
	return nil
}

func resolve_file_format(path string, requested FileFormat) (FileFormat, error) {
	if requested == FormatYAML || requested == FormatJSON {
		return requested, nil
	}
	if requested != FormatAuto {
		return "", fmt.Errorf("configapi: unsupported file format %q", requested)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return FormatJSON, nil
	case ".yaml", ".yml", "":
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("configapi: cannot infer format from %s", path)
	}
}
