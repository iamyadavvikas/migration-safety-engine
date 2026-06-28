// Package frontend handles embedding the compiled React UI assets into the Go binary.
package frontend

import "embed"

// Assets contains the static files built by Vite (HTML, JS, CSS, images).
// We use the "all:" prefix to ensure hidden files (if any) and all subdirectories are recursively embedded.
//
//go:embed all:dist
var Assets embed.FS
