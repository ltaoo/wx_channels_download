package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

func read_config_file(config_filepath string) error {
	extension := strings.ToLower(filepath.Ext(config_filepath))
	if extension != ".yaml" && extension != ".yml" {
		return viper.ReadInConfig()
	}

	config_data, err := os.ReadFile(config_filepath)
	if err != nil {
		return err
	}
	config_data = normalize_yaml_windows_paths(config_data)
	return viper.ReadConfig(bytes.NewReader(config_data))
}

// normalize_yaml_windows_paths makes double-quoted Windows drive paths usable
// in YAML. YAML interprets backslashes in double-quoted values as escapes, but
// Windows users commonly paste paths such as "D:\Videos" verbatim.
func normalize_yaml_windows_paths(config_data []byte) []byte {
	lines := bytes.SplitAfter(config_data, []byte("\n"))
	changed := false
	for line_index, line := range lines {
		normalized_line := normalize_yaml_windows_path_line(line)
		if !bytes.Equal(normalized_line, line) {
			lines[line_index] = normalized_line
			changed = true
		}
	}
	if !changed {
		return config_data
	}
	return bytes.Join(lines, nil)
}

func normalize_yaml_windows_path_line(line []byte) []byte {
	value_start := yaml_double_quoted_value_start(line)
	if value_start < 0 {
		return line
	}
	value_end := yaml_double_quoted_value_end(line, value_start)
	if value_end < 0 {
		return line
	}

	value := line[value_start+1 : value_end]
	if !is_windows_drive_path_literal(value) {
		return line
	}
	normalized_value := escape_unescaped_backslashes(value)
	if bytes.Equal(normalized_value, value) {
		return line
	}

	normalized_line := make([]byte, 0, len(line)+len(normalized_value)-len(value))
	normalized_line = append(normalized_line, line[:value_start+1]...)
	normalized_line = append(normalized_line, normalized_value...)
	normalized_line = append(normalized_line, line[value_end:]...)
	return normalized_line
}

func yaml_double_quoted_value_start(line []byte) int {
	in_single_quote := false
	in_double_quote := false
	escaped := false
	for char_index, char := range line {
		if in_double_quote {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				in_double_quote = false
			}
			continue
		}
		if in_single_quote {
			if char == '\'' {
				in_single_quote = false
			}
			continue
		}
		switch char {
		case '#':
			return -1
		case '\'':
			in_single_quote = true
		case '"':
			in_double_quote = true
		case ':':
			for value_index := char_index + 1; value_index < len(line); value_index++ {
				if line[value_index] == ' ' || line[value_index] == '\t' {
					continue
				}
				if line[value_index] == '"' {
					return value_index
				}
				return -1
			}
		}
	}
	return -1
}

func yaml_double_quoted_value_end(line []byte, value_start int) int {
	backslash_count := 0
	for char_index := value_start + 1; char_index < len(line); char_index++ {
		char := line[char_index]
		if char == '\\' {
			backslash_count++
			continue
		}
		if char == '"' && backslash_count%2 == 0 {
			return char_index
		}
		backslash_count = 0
	}
	return -1
}

func is_windows_drive_path_literal(value []byte) bool {
	if len(value) < 3 {
		return false
	}
	drive_letter := value[0]
	is_ascii_letter := drive_letter >= 'a' && drive_letter <= 'z' || drive_letter >= 'A' && drive_letter <= 'Z'
	return is_ascii_letter && value[1] == ':' && value[2] == '\\'
}

func escape_unescaped_backslashes(value []byte) []byte {
	normalized_value := make([]byte, 0, len(value)+4)
	for char_index := 0; char_index < len(value); {
		if value[char_index] != '\\' {
			normalized_value = append(normalized_value, value[char_index])
			char_index++
			continue
		}

		group_end := char_index
		for group_end < len(value) && value[group_end] == '\\' {
			group_end++
		}
		backslash_count := group_end - char_index
		normalized_value = append(normalized_value, value[char_index:group_end]...)
		if backslash_count%2 != 0 {
			normalized_value = append(normalized_value, '\\')
		}
		char_index = group_end
	}
	return normalized_value
}
