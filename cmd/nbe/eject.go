package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Ejecter struct {
	// Absolute path to the repo.
	Repo string
	// Relative path to the example, examples/ can be omitted.
	Example string
	// Directory to write the example out to.
	Dir string
	// Print out docker build output.
	Verbose bool
	// Defaults to os.Stdout and os.Stderr. Set if these streams need to be
	// explicitly captured.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

func (r *Ejecter) Run() error {
	stdout := r.Stdout
	stderr := r.Stderr

	if stdout == nil {
		stdout = os.Stdout
	}

	if stderr == nil {
		stderr = os.Stderr
	}

	example := r.Example
	if !strings.HasPrefix(example, "examples/") {
		example = filepath.Join("examples", example)
	}

	exampleDir := filepath.Join(r.Repo, example)
	lang := filepath.Base(example)

	defaultDir := filepath.Join(r.Repo, "docker", lang)

	buildDir := r.Dir

	if info, err := os.Stat(buildDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("not a directory: %s", buildDir)
		}
		entries, err := os.ReadDir(buildDir)
		if err != nil {
			return fmt.Errorf("read dir: %w", err)
		}
		if len(entries) > 0 {
			return errors.New("output directory must be empty")
		}
	} else if os.IsNotExist(err) {
		err := os.MkdirAll(buildDir, 0755)
		if err != nil {
			return err
		}
	}

	composeFile := filepath.Join(r.Repo, "docker", "docker-compose.yaml")
	err := copyFile(composeFile, filepath.Join(buildDir, "docker-compose.yaml"))
	if err != nil {
		return fmt.Errorf("copy compoose file: %w", err)
	}

	// Copy default files first.
	if err := copyDirContents(defaultDir, buildDir); err != nil {
		return fmt.Errorf("copy default files: %w", err)
	}

	// Copy example files next..
	if err := copyDirContents(exampleDir, buildDir); err != nil {
		return fmt.Errorf("copy client files: %w", err)
	}

	return nil
}
