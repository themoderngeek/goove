// Package art renders image bytes to terminal-friendly ANSI strings.
//
// The Renderer abstraction has one real implementation (ChafaRenderer, which
// shells out to chafa(1)) and is designed to be injectable for tests. Available()
// reports whether the chafa binary is in PATH; main calls it once at startup
// and skips constructing a renderer when chafa is missing.
package art

import (
	"context"
	"os/exec"
)

// Renderer turns image bytes (PNG/JPEG/anything chafa accepts) into an ANSI
// string suitable for embedding directly in a Bubble Tea View.
type Renderer interface {
	Render(ctx context.Context, image []byte, width, height int) (string, error)
}

// ChafaRunner is the test seam for ChafaRenderer. The real implementation
// (execChafaRunner) shells out to chafa via stdin; tests inject a fake.
type ChafaRunner interface {
	Run(ctx context.Context, image []byte, width, height int) ([]byte, error)
}

// Available reports whether the chafa binary is in PATH.
// Cheap but should be invoked once at startup; main caches the result.
func Available() bool {
	_, err := exec.LookPath("chafa")
	return err == nil
}
