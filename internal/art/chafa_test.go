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
