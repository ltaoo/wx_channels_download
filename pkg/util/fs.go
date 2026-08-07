package util

import (
	"regexp"
)

// ReplaceTemplateVars replaces {{key}} placeholders in the template string with corresponding values from params.
// If a key is not in params, it is replaced with an empty string.
func ReplaceTemplateVars(template string, params map[string]string) string {
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllStringFunc(template, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) > 1 {
			if v, ok := params[sub[1]]; ok {
				return v
			}
		}
		return ""
	})
}
