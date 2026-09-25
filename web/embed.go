package web

import "embed"

// HTML assets are embedded at build time so the API binary serves the UI
// without a separate static host. Replace with a Vite build output here when
// auth, orgs, or a richer SPA are required.
//
//go:embed *.html
var files embed.FS
