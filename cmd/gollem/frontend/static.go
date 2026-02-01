package frontend

import "embed"

//go:embed all:dist
var StaticFiles embed.FS
