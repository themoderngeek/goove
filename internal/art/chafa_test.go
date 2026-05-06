package art

import (
	"context"
	"errors"
	"testing"
)

// fakeChafaRunner records what was piped to chafa and returns scripted output.
type fakeChafaRunner struct {
	gotImage []byte
	gotW     int
	gotH     int
	out      []byte
	err      error
}

func (f *fakeChafaRunner) Run(ctx context.Context, image []byte, width, height int) ([]byte, error) {
	f.gotImage = image
	f.gotW = width
	f.gotH = height
	return f.out, f.err
}

func TestRenderPipesBytesAndDimensionsToRunner(t *testing.T) {
	r := &fakeChafaRunner{out: []byte("ANSI")}
	c := New(r)

	got, err := c.Render(context.Background(), []byte("PNGBYTES"), 20, 10)
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	if got != "ANSI" {
		t.Errorf("got = %q; want %q", got, "ANSI")
	}
	if string(r.gotImage) != "PNGBYTES" {
		t.Errorf("gotImage = %q; want %q", r.gotImage, "PNGBYTES")
	}
	if r.gotW != 20 || r.gotH != 10 {
		t.Errorf("got w/h = %d/%d; want 20/10", r.gotW, r.gotH)
	}
}

func TestRenderRunnerErrorWrapped(t *testing.T) {
	r := &fakeChafaRunner{err: errors.New("boom")}
	c := New(r)

	_, err := c.Render(context.Background(), []byte("x"), 20, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, r.err) {
		t.Errorf("err chain does not contain underlying boom: %v", err)
	}
}

func TestRenderEmptyImageStillReachesRunner(t *testing.T) {
	r := &fakeChafaRunner{out: []byte("")}
	c := New(r)

	got, err := c.Render(context.Background(), []byte{}, 20, 10)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "" {
		t.Errorf("got = %q; want empty", got)
	}
	if r.gotImage == nil {
		t.Errorf("runner did not see the (empty) image")
	}
}

func TestRenderContextCancellationPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel before calling Render

	r := &fakeChafaRunner{err: ctx.Err()}
	c := New(r)

	_, err := c.Render(ctx, []byte("x"), 20, 10)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// chafa surrounds its output with cursor-toggle escapes:
//
//	\x1b[?25l (hide cursor) at the start, \x1b[?25h (show cursor)
//	at the end on its own line followed by a newline.
//
// Without stripping these:
//   - The hide-cursor escape on line 1 inflates lipgloss-measured width of
//     line 1 (escape bytes are not standard ANSI color/style codes that
//     lipgloss strips for width calculation).
//   - The show-cursor escape on its own trailing line means
//     lipgloss.Height(art) returns N+1 instead of N — adding an empty row
//     to any panel layout that sizes itself off the art height.
//
// Both lead to layout glitches in the Now Playing panel. Render must
// strip these.
func TestRenderStripsCursorToggleEscapes(t *testing.T) {
	raw := "\x1b[?25l\x1b[38;5;1mROW1\x1b[0m\nROW2\n\x1b[?25h\n"
	r := &fakeChafaRunner{out: []byte(raw)}
	c := New(r)

	got, err := c.Render(context.Background(), []byte("x"), 20, 10)
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	if got == "" {
		t.Fatalf("got empty; expected stripped art")
	}
	for _, esc := range []string{"\x1b[?25l", "\x1b[?25h"} {
		if i := indexBytes(got, esc); i != -1 {
			t.Errorf("output still contains %q at byte %d: %q", esc, i, got)
		}
	}
}

// indexBytes wraps strings.Index without an extra import.
func indexBytes(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
