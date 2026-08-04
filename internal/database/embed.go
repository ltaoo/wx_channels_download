package database

import "embed"

//go:embed all:migrations
var Migrations embed.FS
