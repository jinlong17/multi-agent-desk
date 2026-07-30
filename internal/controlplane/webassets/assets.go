// Package webassets owns the deterministic, content-hashed P2 Web bundle.
package webassets

import "embed"

// Files contains only the Vite entry document and its hashed JavaScript/CSS.
// It is checked in so fresh-clone Go builds do not depend on a prior Node build.
//
//go:embed index.html assets/*
var Files embed.FS
