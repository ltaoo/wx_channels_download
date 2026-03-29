package database

import "embed"

//go:embed all:migrations
var migrations embed.FS
