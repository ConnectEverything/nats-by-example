package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isRepoRoot reports whether dir looks like the root of a nats-by-example
// checkout.
func isRepoRoot(dir string) bool {
	for _, name := range []string{"examples", "docker"} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !fi.IsDir() {
			return false
		}
	}
	return true
}

// findRepo returns the root of the nats-by-example checkout that contains
// dir, so nbe works from any directory inside the repo, not only the root.
func findRepo(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := dir; ; {
		if isRepoRoot(d) {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("%s is not inside a nats-by-example checkout (no examples/ and docker/ directories found here or above)", dir)
}

// currentRepo is findRepo for the working directory.
func currentRepo() (repo, cwd string, err error) {
	cwd, err = os.Getwd()
	if err != nil {
		return "", "", err
	}
	repo, err = findRepo(cwd)
	return repo, cwd, err
}

// slashPath accepts either separator, so Windows users can pass paths like
// messaging\pub-sub\go, and drops a trailing separator added by shell
// completion.
func slashPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimSuffix(p, "/")
	p = strings.TrimPrefix(p, "./")
	return p
}

// resolveExample turns an example argument into a slash-separated path
// relative to the repo root, starting with "examples/". The argument may be
// relative to the examples/ directory (messaging/pub-sub/go), to the repo
// root (examples/messaging/pub-sub/go), or to the working directory when it
// is somewhere inside examples/.
func resolveExample(repo, cwd, arg string) string {
	a := slashPath(arg)

	// Relative to (or an absolute path inside) the working directory.
	p := filepath.FromSlash(a)
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		if rel, err := filepath.Rel(repo, p); err == nil {
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "examples/") {
				return rel
			}
		}
	}

	return examplePath(a)
}

// checkExampleDirs returns a readable error when the client directory or its
// docker/<client> defaults are missing, instead of a low-level file error.
func checkExampleDirs(repo, example string) error {
	clientDir := filepath.Join(repo, filepath.FromSlash(example))
	if fi, err := os.Stat(clientDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("example %q not found: expected a directory at %s", example, clientDir)
	}
	lang := filepath.Base(clientDir)
	defaultDir := filepath.Join(repo, "docker", lang)
	if fi, err := os.Stat(defaultDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("%q is not a client directory: no docker/%s directory in %s (example paths look like messaging/pub-sub/go)", example, lang, repo)
	}
	return nil
}

// examplePath normalizes an example path that is relative to the repo root
// or to examples/ into the slash-separated "examples/..." form.
func examplePath(example string) string {
	e := slashPath(example)
	if strings.HasPrefix(e, "examples/") {
		return e
	}
	return "examples/" + e
}
