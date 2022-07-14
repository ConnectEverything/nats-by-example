package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

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

func generateOutput(repo, example string, recreate bool) error {
	p := filepath.Join(repo, example, "output.txt")

	_, err := os.Stat(p)
	// Exists..
	if err == nil {
		// Regardless if it was generated or manually created.
		if !recreate {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("open file: %w", err)
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	r := ExampleRunner{
		Repo:    repo,
		Example: example,
		Stdout:  stdout,
		Stderr:  stderr,
	}

	err = r.Run()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, stderr.String())
	}

	// Create new or recreate.
	err = createFile(p, stdout.Bytes())
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	return nil
}

type ExampleRunner struct {
	Name string
	// Absolute path to the repo.
	Repo string
	// Relative path to the example, examples/ can be omitted.
	Example string
	// Set to true, to force the use of a cluster.
	Cluster bool
	// If true, do not delete the image.
	KeepImage bool
	// Defaults to os.Stdout and os.Stderr. Set if these streams need to be
	// explicitly captured.
	Stdout io.Writer
	Stderr io.Writer
}

func (r *ExampleRunner) Run() error {
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

	imageTag := fmt.Sprintf("%s:%s", filepath.Join("nbe", r.Example), uid)

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

	// Build the temporary image relative to the build directory.
	c := exec.Command(
		"docker",
		"build",
		"--tag", imageTag,
		buildDir,
	)

	c.Stdout = stdout
	c.Stderr = stderr

	err = c.Run()
	if err != nil {
		return fmt.Errorf("build image: %w", err)
	}

	if !r.KeepImage {
		// Remove the built image on exit to prevent
		// TODO: should rely on git hash instead of random uid.
		defer exec.Command(
			"docker",
			"rmi",
			imageTag,
		).Run()
	}

	// Create a temporary directory as the project directory for the temporary
	// .env file containing the image tag.
	composeDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	// Clean up the directory on exit.
	defer os.RemoveAll(composeDir)

	err = createFile(filepath.Join(composeDir, ".env"), []byte(fmt.Sprintf("IMAGE_TAG=%s", imageTag)))
	if err != nil {
		return fmt.Errorf("create .env: %w", err)
	}

	// Best effort to bring containers down..
	defer exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", composeDir,
		"--file", composeFile,
		"down",
		"--remove-orphans",
		"--timeout", "3",
	).Run()

	// Run the app container.
	cmd := exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", composeDir,
		"--file", composeFile,
		"run",
		"--no-TTY",
		"--rm",
		"app",
	)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}
