package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func copyFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	dir := filepath.Dir(dst)
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}

	c, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(c, f)
	if err != nil {
		return err
	}
	return c.Close()
}

func createFile(n string, b []byte) error {
	dir := filepath.Dir(n)
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}

	c, err := os.Create(n)
	if err != nil {
		return err
	}
	_, err = c.Write(b)
	if err != nil {
		return err
	}
	return c.Close()
}

func copyDirContents(src, dst string) error {
	return fs.WalkDir(os.DirFS(src), ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		dstpath := filepath.Join(dst, path)
		// Ensure any directories are created..
		if info.IsDir() {
			return os.MkdirAll(dstpath, 0755)
		}
		sf, err := os.Open(filepath.Join(src, path))
		if err != nil {
			return err
		}
		defer sf.Close()
		df, err := os.Create(dstpath)
		if err != nil {
			return err
		}
		_, err = io.Copy(df, sf)
		return err
	})
}

type ImageBuilder struct {
	Name string
	// Absolute path to the repo.
	Repo string
	// Relative path to the example, examples/ can be omitted.
	Example string
	// Print out docker build output.
	Verbose bool
	// Defaults to os.Stdout and os.Stderr. Set if these streams need to be
	// explicitly captured.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

func (r *ImageBuilder) Run() (string, error) {
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

	clientDir := filepath.Join(r.Repo, example)
	lang := filepath.Base(example)

	var uid string
	if r.Name != "" {
		uid = r.Name
	} else {
		uid = uuid.New().String()[:8]
	}

	imageTag := fmt.Sprintf("%s:%s", filepath.Join("nbe", r.Example), uid)

	defaultDir := filepath.Join(r.Repo, "docker", lang)

	// Create a temporary directory for the build context of the image.
	// This will combine all files in the runtime-specific docker/ directory
	// and the files in the example.
	buildDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", fmt.Errorf("temp dir: %w", err)
	}
	// Clean up the directory on exit.
	defer os.RemoveAll(buildDir)

	// Copy default files first.
	if err := copyDirContents(defaultDir, buildDir); err != nil {
		return "", fmt.Errorf("copy default files: %w", err)
	}

	// Copy example files next..
	if err := copyDirContents(clientDir, buildDir); err != nil {
		return "", fmt.Errorf("copy client files: %w", err)
	}

	// Build the temporary image relative to the build directory.
	c := exec.Command(
		"docker",
		"build",
		"--tag", imageTag,
		buildDir,
	)

	if r.Verbose {
		c.Stdout = stdout
	}
	c.Stderr = stderr

	err = c.Run()
	if err != nil {
		return "", fmt.Errorf("build image: %w", err)
	}

	return imageTag, nil
}

func removeImage(image string) error {
	c := exec.Command("docker", "rmi", image)
	return c.Run()
}

type ComposeRunner struct {
	Name string
	// Absolute path to the repo.
	Repo string
	// Relative path to the example, examples/ can be omitted.
	Example string
	// Set to true, to force the use of a cluster.
	Cluster bool
	// If true, do not delete the image.
	Keep bool
	// If true, use "compose up" instead of "run"
	Up bool
	// Print out docker build output.
	Verbose bool
	// If true, do not use ansi control characters.
	NoAnsi bool
	// Defaults to os.Stdout and os.Stderr. Set if these streams need to be
	// explicitly captured.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

func (r *ComposeRunner) Run(imageTag string) error {
	stdout := r.Stdout
	stderr := r.Stderr
	stdin := r.Stdin

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

	clientDir := filepath.Join(r.Repo, example)
	exampleDir := filepath.Dir(clientDir)
	lang := filepath.Base(example)

	composeFile := filepath.Join(exampleDir, "docker-compose.yaml")
	if _, err := os.Stat(composeFile); err != nil {
		if os.IsNotExist(err) {
			if r.Cluster {
				composeFile = filepath.Join(r.Repo, "docker", "docker-compose.cluster.yaml")
			} else {
				composeFile = filepath.Join(r.Repo, "docker", "docker-compose.yaml")
			}
		} else {
			return err
		}
	}

	var uid string
	if r.Name != "" {
		uid = r.Name
	} else {
		uid = uuid.New().String()[:8]
	}

	defaultDir := filepath.Join(r.Repo, "docker", lang)

	// Create a temporary directory for the build context of the image.
	// This will combine all files in the runtime-specific docker/ directory
	// and the files in the example.
	buildDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	// Clean up the directory on exit.
	defer os.RemoveAll(buildDir)

	// Copy default files first.
	if err := copyDirContents(defaultDir, buildDir); err != nil {
		return err
	}

	// Copy example files next..
	if err := copyDirContents(clientDir, buildDir); err != nil {
		return err
	}

	err = copyFile(composeFile, filepath.Join(buildDir, "docker-compose.yaml"))
	if err != nil {
		return err
	}

	err = createFile(filepath.Join(buildDir, ".env"), []byte(fmt.Sprintf("IMAGE_TAG=%s", imageTag)))
	if err != nil {
		return fmt.Errorf("create .env: %w", err)
	}

	// Best effort to bring containers down..
	defer func() {
		cmd := exec.Command(
			"docker",
			"compose",
			"--project-name", uid,
			"down",
			"--remove-orphans",
			"--timeout", "3",
		)
		cmd.Dir = buildDir
		cmd.Run()
	}()

	cmd := exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"pull",
		"--include-deps",
		"--quiet",
		"--ignore-pull-failures",
	)
	cmd.Dir = buildDir

	stderrb := bytes.NewBuffer(nil)
	cmd.Stderr = stderrb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull images: %w\n%s", err, stderrb.String())
	}

	if r.Up {
		// Run the app container.
		cmd = exec.Command(
			"docker",
			"compose",
			"--project-name", uid,
			"up",
		)
	} else {
		ansi := "auto"
		if r.NoAnsi {
			ansi = "never"
		}

		// Run the app container.
		cmd = exec.Command(
			"docker",
			"compose",
			"--ansi", ansi,
			"--project-name", uid,
			"run",
			"--no-TTY",
			"--rm",
			"app",
		)
	}

	cmd.Dir = buildDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin

	done := make(chan error, 1)
	sigch := make(chan os.Signal, 1)

	signal.Notify(sigch, os.Interrupt)

	go func() {
		done <- cmd.Run()
	}()

	// Wait for interrupt or once the command finishes.
	// This ensures the `docker compose down` runs.
	select {
	case err := <-done:
		return err
	case <-sigch:
		return nil
	}
}
