// Package web embeds the static frontend assets (HTML, CSS) so the
// compiled binary is fully self-contained.
package web

import "embed"

//go:embed index.html style.css logo.png
var Content embed.FS
