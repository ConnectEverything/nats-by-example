package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, d := range []string{
		"docker/go",
		"examples/messaging/pub-sub/go",
	} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(d)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestFindRepoFromSubdirectory(t *testing.T) {
	repo := makeRepo(t)
	for _, dir := range []string{"", "examples", "examples/messaging/pub-sub/go"} {
		got, err := findRepo(filepath.Join(repo, filepath.FromSlash(dir)))
		if err != nil {
			t.Fatalf("%q: %v", dir, err)
		}
		if got != repo {
			t.Errorf("%q: got %s, want %s", dir, got, repo)
		}
	}
	if _, err := findRepo(t.TempDir()); err == nil {
		t.Error("expected an error outside a checkout")
	}
}

func TestResolveExample(t *testing.T) {
	repo := makeRepo(t)
	at := func(d string) string { return filepath.Join(repo, filepath.FromSlash(d)) }
	want := "examples/messaging/pub-sub/go"
	cases := []struct{ cwd, arg string }{
		{"", "messaging/pub-sub/go"},
		{"", "messaging/pub-sub/go/"},
		{"", `messaging\pub-sub\go`},
		{"", "examples/messaging/pub-sub/go"},
		{"", `examples\messaging\pub-sub\go\`},
		{"", "./examples/messaging/pub-sub/go"},
		{"examples", "messaging/pub-sub/go"},
		{"examples/messaging", "pub-sub/go"},
		{"examples/messaging/pub-sub/go", "."},
		{"docker", "messaging/pub-sub/go"},
		{"", at("examples/messaging/pub-sub/go")},
	}
	for _, c := range cases {
		if got := resolveExample(repo, at(c.cwd), c.arg); got != want {
			t.Errorf("cwd %q, arg %q: got %q, want %q", c.cwd, c.arg, got, want)
		}
	}
}

func TestCheckExampleDirs(t *testing.T) {
	repo := makeRepo(t)
	if err := checkExampleDirs(repo, "examples/messaging/pub-sub/go"); err != nil {
		t.Fatal(err)
	}
	err := checkExampleDirs(repo, "examples/messaging/pub-sub/rust")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing example: got %v", err)
	}
	err = checkExampleDirs(repo, "examples/messaging/pub-sub")
	if err == nil || !strings.Contains(err.Error(), "not a client directory") {
		t.Errorf("non-client directory: got %v", err)
	}
}
