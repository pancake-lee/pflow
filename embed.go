// Package pflow embeds the web dashboard static files for single-binary deployment.
package pflow

import "embed"

// WebDist holds the built Vue SPA files from web/dist.
//
//go:embed web/dist/*
var WebDist embed.FS
