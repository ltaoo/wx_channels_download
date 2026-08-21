package cookies

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var json_file_mu sync.Mutex

// SaveJSON writes cookies to the application's persistent JSON cookie file.
func SaveJSON(cookie_list []Cookie, path string) error {
	json_file_mu.Lock()
	defer json_file_mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("cookies: create output directory: %w", err)
	}
	data, err := json.MarshalIndent(cookie_list, "", "  ")
	if err != nil {
		return fmt.Errorf("cookies: marshal JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cookies: write JSON: %w", err)
	}
	return nil
}
