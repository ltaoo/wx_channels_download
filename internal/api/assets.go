package api

import (
	_ "embed"
)

//go:embed ui/index.html
var html_home []byte

type Assets struct {
	HTMLHome []byte
}

// var files = &Assets{
// 	HTMLHome: html_home,
// }
