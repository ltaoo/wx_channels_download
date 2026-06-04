package youtube

import "testing"

func TestExtractVideoID(t *testing.T) {
	tests := map[string]string{
		"https://www.youtube.com/watch?v=abc123": "abc123",
		"https://youtu.be/xyz789":                "xyz789",
		"https://www.youtube.com/shorts/short1":  "short1",
	}
	for rawURL, want := range tests {
		got, ok := ExtractVideoID(rawURL)
		if !ok {
			t.Fatalf("ExtractVideoID(%q) returned false", rawURL)
		}
		if got != want {
			t.Fatalf("ExtractVideoID(%q) = %q, want %q", rawURL, got, want)
		}
	}
}
