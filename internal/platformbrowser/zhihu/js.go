package zhihu

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed scripts/*.js
var scriptsFS embed.FS

func Script() string {
	return strings.Join(Scripts(), "\n;\n")
}

func Scripts() []string {
	entries, err := scriptsFS.ReadDir("scripts")
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var scripts []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		data, err := scriptsFS.ReadFile("scripts/" + entry.Name())
		if err != nil {
			continue
		}
		scripts = append(scripts, fmt.Sprintf("\n/* zhihu/%s */\n%s", entry.Name(), string(data)))
	}
	return scripts
}
