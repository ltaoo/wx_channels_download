package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadDefaultEnv(repoRoot string) error {
	err := loadEnvFile(filepath.Join(repoRoot, ".env"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		key, value, ok, err := parseEnvLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
	}
	return scanner.Err()
}

func parseEnvLine(line string) (string, string, bool, error) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}

	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", "", false, fmt.Errorf("invalid .env line")
	}
	key := strings.TrimSpace(line[:eq])
	if !validEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid environment variable name %q", key)
	}

	value, err := parseEnvValue(strings.TrimSpace(line[eq+1:]))
	if err != nil {
		return "", "", false, err
	}
	return key, value, true, nil
}

func parseEnvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	switch raw[0] {
	case '"', '\'':
		value, rest, quote, err := parseQuotedEnvValue(raw)
		if err != nil {
			return "", err
		}
		rest = strings.TrimSpace(rest)
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", fmt.Errorf("unexpected trailing content after quoted value")
		}
		if quote == '"' {
			value = os.ExpandEnv(value)
		}
		return value, nil
	default:
		return os.ExpandEnv(stripEnvInlineComment(raw)), nil
	}
}

func parseQuotedEnvValue(raw string) (string, string, byte, error) {
	quote := raw[0]
	var value strings.Builder
	for i := 1; i < len(raw); i++ {
		ch := raw[i]
		if ch == quote {
			return value.String(), raw[i+1:], quote, nil
		}
		if quote == '"' && ch == '\\' && i+1 < len(raw) {
			i++
			switch raw[i] {
			case 'n':
				value.WriteByte('\n')
			case 'r':
				value.WriteByte('\r')
			case 't':
				value.WriteByte('\t')
			default:
				value.WriteByte(raw[i])
			}
			continue
		}
		value.WriteByte(ch)
	}
	return "", "", quote, fmt.Errorf("unterminated quoted value")
}

func stripEnvInlineComment(value string) string {
	for i := 0; i < len(value); i++ {
		if value[i] != '#' {
			continue
		}
		if i == 0 || value[i-1] == ' ' || value[i-1] == '\t' {
			return strings.TrimSpace(value[:i])
		}
	}
	return strings.TrimSpace(value)
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		ch := key[i]
		if ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
			continue
		}
		if i > 0 && ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}
