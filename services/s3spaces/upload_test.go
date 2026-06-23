package s3spaces

import (
	"bytes"
	"strings"
	"testing"
)

// fakeFile is a minimal multipart.File backed by a bytes.Reader.
type fakeFile struct {
	*bytes.Reader
}

func (fakeFile) Close() error { return nil }

func newFakeFile(b []byte) fakeFile { return fakeFile{bytes.NewReader(b)} }

// pngHeader is a minimal valid PNG signature that http.DetectContentType
// recognises as image/png.
var pngHeader = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}

func TestDetectImageExtAllowsPNG(t *testing.T) {
	f := newFakeFile(pngHeader)
	ext, err := detectImageExt(f)
	if err != nil {
		t.Fatalf("detectImageExt: %v", err)
	}
	if ext != ".png" {
		t.Fatalf("ext: got %q want .png", ext)
	}
	// File offset must be reset so the full content can be persisted.
	if pos, _ := f.Seek(0, 1); pos != 0 {
		t.Fatalf("file offset not reset: %d", pos)
	}
}

func TestDetectImageExtRejectsNonImage(t *testing.T) {
	f := newFakeFile([]byte("<html><script>alert(1)</script></html>"))
	if _, err := detectImageExt(f); err == nil {
		t.Fatal("expected non-image to be rejected")
	}
}

func TestBuildObjectKeyServerControlled(t *testing.T) {
	key, err := buildObjectKey("products/store-1", ".png")
	if err != nil {
		t.Fatalf("buildObjectKey: %v", err)
	}
	if !strings.HasPrefix(key, "products/store-1/") {
		t.Fatalf("key prefix wrong: %q", key)
	}
	if !strings.HasSuffix(key, ".png") {
		t.Fatalf("key suffix wrong: %q", key)
	}
	// The uuid component must not contain the client-supplied filename.
	if strings.Contains(key, "..") {
		t.Fatalf("key contains traversal: %q", key)
	}
}

func TestBuildObjectKeyRejectsEmptyOrRoot(t *testing.T) {
	cases := []string{"..", "/", "", ".", "   "}
	for _, c := range cases {
		if _, err := buildObjectKey(c, ".png"); err == nil {
			t.Fatalf("expected %q to be rejected", c)
		}
	}
}

func TestBuildObjectKeyNeutralisesTraversal(t *testing.T) {
	// path.Clean anchors to root, so embedded "../" segments collapse and can
	// never escape the bucket namespace.
	key, err := buildObjectKey("evidence/../../secret", ".png")
	if err != nil {
		t.Fatalf("buildObjectKey: %v", err)
	}
	if !strings.HasPrefix(key, "secret/") {
		t.Fatalf("traversal not neutralised, got %q", key)
	}
	if strings.Contains(key, "..") {
		t.Fatalf("key still contains traversal: %q", key)
	}
}
