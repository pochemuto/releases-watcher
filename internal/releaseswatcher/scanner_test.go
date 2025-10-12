package releaseswatcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/dhowden/tag"
)

func TestScan(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("EXCLUDED_PATH", filepath.Join(tmp, "skip"))
	os.Mkdir(filepath.Join(tmp, "skip"), 0o755)
	os.WriteFile(filepath.Join(tmp, "a.mp3"), []byte("dummy"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.m4a"), []byte("dummy"), 0o644)
	os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("dummy"), 0o644)

	ch := make(chan string, 10)
	var counter atomic.Int32
	err := Scan(context.Background(), tmp, ch, &counter)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	var files []string
	for f := range ch {
		files = append(files, filepath.Base(f))
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %v", files)
	}
	if counter.Load() != 2 {
		t.Errorf("expected counter 2, got %d", counter.Load())
	}
}

type fakeReadSeekerCloser struct {
	*bytes.Reader
	closed bool
}

func (f *fakeReadSeekerCloser) Close() error { f.closed = true; return nil }

// Patch points for testing
var (
	osOpen      = os.Open
	tagReadFrom = tag.ReadFrom
)

func TestReadID3(t *testing.T) {
	// Patch os.Open and tag.ReadFrom
	oldOpen := osOpen
	oldRead := tagReadFrom
	defer func() { osOpen = oldOpen; tagReadFrom = oldRead }()

	fake := &fakeReadSeekerCloser{Reader: bytes.NewReader([]byte("test"))}
	osOpen = func(name string) (*os.File, error) {
		return nil, errors.New("not a real file") // never called in this test
	}
	readID3Test := func(filepath string) (tag.Metadata, error) {
		var r io.ReadSeeker
		if filepath == "ok.mp3" {
			r = fake
		} else {
			return nil, errors.New("not found")
		}
		t, err := tagReadFrom(r)
		if err != nil {
			return nil, err
		}
		fake.Close()
		return t, nil
	}
	tagReadFrom = func(r io.ReadSeeker) (tag.Metadata, error) {
		if _, ok := r.(*fakeReadSeekerCloser); ok {
			return tag.Metadata(nil), nil
		}
		return nil, errors.New("bad reader")
	}

	_, err := readID3Test("ok.mp3")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	_, err = readID3Test("fail.mp3")
	if err == nil {
		t.Errorf("expected error for fail.mp3")
	}
}
