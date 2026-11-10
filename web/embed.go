// Package web holds the browser interface, embedded into the executable.
package web

import "embed"

//go:embed static templates
var Files embed.FS
