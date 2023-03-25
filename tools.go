//go:build tools

// See:
// - https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// - https://play-with-go.dev/tools-as-dependencies_go119_en/

package tools

import (
	_ "honnef.co/go/tools/cmd/staticcheck"
)
