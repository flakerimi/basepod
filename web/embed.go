// Package web embeds the built Vue SPA so the server can ship as a single binary.
//
// Run `make web` (which runs `pnpm install && pnpm build`) before building the
// server, or the embedded filesystem will be empty.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
