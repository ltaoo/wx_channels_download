package api

import (
	_ "embed"
)

//go:embed ui/index.html
var html_home []byte

//go:embed ui/preview.html
var preview_home []byte

type Assets struct {
	HTMLHome    []byte
	HTMLPreview []byte
}

var files = &Assets{
	HTMLHome:    html_home,
	HTMLPreview: preview_home,
}
