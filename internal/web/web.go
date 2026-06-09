package web

import "embed"

// Static contains the browser UI served by the API process.
//
//go:embed static
var Static embed.FS
