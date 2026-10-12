package web

import "embed"

// Assets contains the production Vue build. Build the frontend before releasing Go.
//
//go:embed all:dist
var Assets embed.FS
