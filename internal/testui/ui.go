package testui

import (
	"embed"
	"fmt"
)

//go:embed tests.html
var testUI embed.FS

// TestUIPage returns the embedded test management HTML page.
func TestUIPage() (string, error) {
	data, err := testUI.ReadFile("tests.html")
	if err != nil {
		return "", fmt.Errorf("read tests.html: %w", err)
	}
	return string(data), nil
}
