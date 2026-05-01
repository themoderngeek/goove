//go:build integration

package art

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegrationChafaRendersFixture(t *testing.T) {
	if !Available() {
		t.Skip("chafa not in PATH; install with `brew install chafa`")
	}

	fixture, err := os.ReadFile(filepath.Join("testdata", "fixture.png"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	r := NewChafaRenderer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := r.Render(ctx, fixture, 20, 10)
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	if len(got) == 0 {
		t.Fatal("Render returned empty string; expected ANSI output")
	}
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("output does not contain ANSI escapes: %q", got[:min(80, len(got))])
	}
	t.Logf("chafa rendered %d bytes of ANSI output", len(got))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
