package webui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestFS_HasIndex(t *testing.T) {
	b, err := fs.ReadFile(FS(), "index.html")
	if err != nil {
		t.Fatalf("index.html: %v", err)
	}
	if !strings.Contains(string(b), "<!doctype html>") {
		t.Errorf("index.html does not look like HTML: %.80q", b)
	}
}

func TestPlaceholder_IsCommittedAndLinksSignIn(t *testing.T) {
	b, err := fs.ReadFile(dist, "dist/placeholder/index.html")
	if err != nil {
		t.Fatalf("placeholder: %v", err)
	}
	if !strings.Contains(string(b), `href="/auth/login"`) || !strings.Contains(string(b), "make web") {
		t.Errorf("placeholder must link sign-in and explain the build: %s", b)
	}
}

func TestBuilt_MatchesFS(t *testing.T) {
	_, err := fs.Stat(dist, "dist/app/index.html")
	if Built() != (err == nil) {
		t.Errorf("Built() = %v but app/index.html stat err = %v", Built(), err)
	}
}
