package assetsync

import (
	"path"
	"regexp"
	"strings"
)

func matchAny(patterns []string, rel string) bool {
	rel = cleanSlashPath(rel)
	for _, pattern := range patterns {
		if globMatch(pattern, rel) {
			return true
		}
	}
	return false
}

func globMatch(pattern, rel string) bool {
	pattern = cleanSlashPath(pattern)
	rel = cleanSlashPath(rel)
	if pattern == "**/*" || pattern == "**" {
		return rel != "."
	}
	if ok, err := path.Match(pattern, rel); err == nil && ok {
		return true
	}
	re, err := regexp.Compile(globToRegexp(pattern))
	if err != nil {
		return false
	}
	return re.MatchString(rel)
}

func globToRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteString("$")
	return b.String()
}
