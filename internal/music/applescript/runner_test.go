//go:build darwin

package applescript

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOsascriptRunnerRunsTrivialScript(t *testing.T) {
	r := OsascriptRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := r.Run(ctx, `return "hello"`)
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if strings.TrimSpace(string(out)) != "hello" {
		t.Fatalf("out = %q; want %q", strings.TrimSpace(string(out)), "hello")
	}
}
