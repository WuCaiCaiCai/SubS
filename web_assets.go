package subs

import "embed"

// WebAssets carries the built SPA. The `all:` prefix keeps dotfiles in the
// build so a placeholder can satisfy the embed directive before the frontend is
// compiled (see web/dist/.gitkeep).
//
//go:embed all:web/dist
var WebAssets embed.FS
