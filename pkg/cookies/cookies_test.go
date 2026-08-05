package cookies

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCookieToHTTP(t *testing.T) {
	c := Cookie{
		Name:     "session",
		Value:    "abc123",
		Domain:   "example.com",
		Path:     "/",
		Secure:   true,
		HTTPOnly: true,
		SameSite: "Strict",
		Expires:  2000000000,
	}

	hc := c.ToHTTP()
	if hc.Name != "session" {
		t.Errorf("Name = %q, want %q", hc.Name, "session")
	}
	if hc.Value != "abc123" {
		t.Errorf("Value = %q, want %q", hc.Value, "abc123")
	}
	if hc.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", hc.Domain, "example.com")
	}
	if !hc.Secure {
		t.Error("Secure should be true")
	}
	if !hc.HttpOnly {
		t.Error("HttpOnly should be true")
	}
	if hc.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want SameSiteStrictMode", hc.SameSite)
	}
	if hc.Expires.Unix() != 2000000000 {
		t.Errorf("Expires = %d, want 2000000000", hc.Expires.Unix())
	}
}

func TestCookieToHeader(t *testing.T) {
	c := Cookie{Name: "token", Value: "xyz"}
	if got := c.ToHeader(); got != "token=xyz" {
		t.Errorf("ToHeader = %q, want %q", got, "token=xyz")
	}
}

func TestFromHTTP(t *testing.T) {
	expires := time.Unix(2000000000, 0)
	hc := &http.Cookie{
		Name:     "session",
		Value:    "abc123",
		Domain:   "example.com",
		Path:     "/app",
		Secure:   true,
		HttpOnly: true,
		Expires:  expires,
		SameSite: http.SameSiteLaxMode,
	}

	c := FromHTTP(hc)
	if c.Name != "session" {
		t.Errorf("Name = %q, want session", c.Name)
	}
	if c.Value != "abc123" {
		t.Errorf("Value = %q, want abc123", c.Value)
	}
	if c.Domain != "example.com" {
		t.Errorf("Domain = %q, want example.com", c.Domain)
	}
	if c.Path != "/app" {
		t.Errorf("Path = %q, want /app", c.Path)
	}
	if !c.Secure {
		t.Error("Secure should be true")
	}
	if !c.HTTPOnly {
		t.Error("HTTPOnly should be true")
	}
	if c.Expires != 2000000000 {
		t.Errorf("Expires = %d, want 2000000000", c.Expires)
	}
	if c.SameSite != "Lax" {
		t.Errorf("SameSite = %q, want Lax", c.SameSite)
	}
}

func TestFromHTTPSessionCookie(t *testing.T) {
	hc := &http.Cookie{
		Name:  "session",
		Value: "abc",
	}
	c := FromHTTP(hc)
	if c.Expires != -1 {
		t.Errorf("Expires = %d, want -1 for session cookie", c.Expires)
	}
	if c.SameSite != "" {
		t.Errorf("SameSite = %q, want empty for default", c.SameSite)
	}
}

func TestParseSetCookies(t *testing.T) {
	headers := []string{
		"session=abc; Path=/; Domain=example.com; Secure; HttpOnly; SameSite=Lax",
		"token=xyz; Path=/api; Domain=example.com",
	}

	cookies := ParseSetCookies(headers)
	if len(cookies) != 2 {
		t.Fatalf("len = %d, want 2", len(cookies))
	}

	if cookies[0].Name != "session" || cookies[0].Value != "abc" {
		t.Errorf("cookie[0] = %s=%s, want session=abc", cookies[0].Name, cookies[0].Value)
	}
	if cookies[1].Name != "token" || cookies[1].Value != "xyz" {
		t.Errorf("cookie[1] = %s=%s, want token=xyz", cookies[1].Name, cookies[1].Value)
	}
}

func TestParseCookieHeader(t *testing.T) {
	u, _ := url.Parse("https://example.com/path")
	cookies := ParseCookieHeader("session=abc; token=xyz", u)

	if len(cookies) != 2 {
		t.Fatalf("len = %d, want 2", len(cookies))
	}
	if cookies[0].Name != "session" || cookies[0].Value != "abc" {
		t.Errorf("cookie[0] = %s=%s, want session=abc", cookies[0].Name, cookies[0].Value)
	}
	if cookies[1].Name != "token" || cookies[1].Value != "xyz" {
		t.Errorf("cookie[1] = %s=%s, want token=xyz", cookies[1].Name, cookies[1].Value)
	}
}

func TestParseCookieHeaderNilURL(t *testing.T) {
	cookies := ParseCookieHeader("a=1", nil)
	if cookies != nil {
		t.Errorf("expected nil for nil URL, got %v", cookies)
	}
}

func TestFormatCookieHeader(t *testing.T) {
	cookies := []Cookie{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	got := FormatCookieHeader(cookies)
	if got != "a=1; b=2" {
		t.Errorf("FormatCookieHeader = %q, want %q", got, "a=1; b=2")
	}
}

func TestFormatSetCookie(t *testing.T) {
	c := Cookie{
		Name:   "session",
		Value:  "abc",
		Path:   "/",
		Domain: "example.com",
		Secure: true,
	}
	got := FormatSetCookie(c)
	if got == "" {
		t.Error("FormatSetCookie returned empty string")
	}
	if !contains(got, "session=abc") {
		t.Errorf("expected 'session=abc' in %q", got)
	}
}

func TestJarSetAndGet(t *testing.T) {
	jar := NewJar()
	u, _ := url.Parse("https://example.com/")

	jar.SetCookies(u, []*http.Cookie{
		{Name: "a", Value: "1", Path: "/"},
		{Name: "b", Value: "2", Path: "/"},
	})

	got := jar.Cookies(u)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestJarSetCookieFromHeader(t *testing.T) {
	jar := NewJar()
	u, _ := url.Parse("https://example.com/")

	jar.SetCookieFromHeader(u, "session=abc; Path=/; Domain=example.com")
	got := jar.Cookies(u)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Name != "session" || got[0].Value != "abc" {
		t.Errorf("cookie = %s=%s, want session=abc", got[0].Name, got[0].Value)
	}
}

func TestJarSetCookieFromString(t *testing.T) {
	jar := NewJar()
	u, _ := url.Parse("https://example.com/")

	jar.SetCookieFromString(u, "x=1; y=2; z=3")
	got := jar.Cookies(u)

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}

func TestLoadFromNetscape(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cookies.txt")

	content := `# Netscape HTTP Cookie File
# http://example.com/

.example.com	TRUE	/	FALSE	2147483647	session	abc123
.example.com	TRUE	/	TRUE	2147483647	token	xyz789
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	jar := NewJar()
	n, err := jar.LoadFromNetscape(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("loaded %d cookies, want 2", n)
	}

	u, _ := url.Parse("https://example.com/")
	got := jar.Cookies(u)
	if len(got) != 2 {
		t.Errorf("got %d cookies for example.com, want 2", len(got))
	}
}

func TestLoadFromNetscapeNotExist(t *testing.T) {
	jar := NewJar()
	n, err := jar.LoadFromNetscape("/nonexistent/cookies.txt")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

func TestExtractFromResponse(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Set-Cookie": {
				"a=1; Path=/; Domain=example.com",
				"b=2; Path=/; Domain=example.com; Secure",
			},
		},
	}

	cookies := ExtractFromResponse(resp)
	if len(cookies) != 2 {
		t.Fatalf("len = %d, want 2", len(cookies))
	}
	if cookies[0].Name != "a" || cookies[1].Name != "b" {
		t.Errorf("cookies = %v", cookies)
	}
}

func TestParseNetscapeLineEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		valid bool
	}{
		{"standard", ".example.com\tTRUE\t/\tTRUE\t2147483647\tsid\tabc", true},
		{"too_few_fields", "example.com\tTRUE\t/", false},
		{"empty_line", "", false},
		{"comment", "# comment line", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseNetscapeLine(tt.line)
			if ok != tt.valid {
				t.Errorf("parseNetscapeLine(%q) ok = %v, want %v", tt.line, ok, tt.valid)
			}
		})
	}
}

func TestJarSaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cookies.txt")

	jar := NewJar()
	err := jar.SaveToFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file was not created")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
