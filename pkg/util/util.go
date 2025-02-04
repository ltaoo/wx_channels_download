package util

import "strings"

func Includes(str, substr string) bool {
	return strings.Contains(str, substr)
}
