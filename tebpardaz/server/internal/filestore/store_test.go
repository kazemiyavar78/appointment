package filestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAbsoluteRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Absolute("../secret.txt"); err == nil {
		t.Fatal("expected traversal to fail")
	}
	got, err := store.Absolute("lab/2026-09/1/file.pdf")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "lab", "2026-09", "1", "file.pdf")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAdImageStaysInsideAdsDir(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdImage(`..\x.png`); err == nil {
		t.Fatal("expected bad name to fail")
	}
	got, err := store.AdImage("photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root, "ads", "photo.png") {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}
