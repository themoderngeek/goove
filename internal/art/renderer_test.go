package art

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAvailableReturnsFalseWhenChafaMissing(t *testing.T) {
	// Point PATH at an empty directory — chafa cannot be found.
	t.Setenv("PATH", t.TempDir())
	if Available() {
		t.Fatal("Available() = true; expected false with empty PATH")
	}
}

func TestAvailableReturnsTrueWhenChafaInPath(t *testing.T) {
	// Drop a stub executable named "chafa" into a temp dir; point PATH there.
	dir := t.TempDir()
	stub := filepath.Join(dir, "chafa")
	if err := writeExecutable(stub, "#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	t.Setenv("PATH", dir)
	if !Available() {
		t.Fatal("Available() = false; expected true with chafa stub on PATH")
	}
}

func writeExecutable(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o755)
}
