package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

	path := filepath.Join(r.Repo, example)

	lang := filepath.Base(example)

	dockerfile := filepath.Join(path, "Dockerfile")
	if _, err := os.Stat(dockerfile); err != nil {
		if os.IsNotExist(err) {
			dockerfile = filepath.Join(r.Repo, "docker", lang, "Dockerfile")
		} else {
			return err
		}
	}

	if _, err := os.Stat(dockerfile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no default Dockerfile currently exists for %q", lang)
		} else {
			return err
		}
	}

	composefile := filepath.Join(path, "docker-compose.yaml")
	if _, err := os.Stat(composefile); err != nil {
		if os.IsNotExist(err) {
			if r.Cluster {
				composefile = filepath.Join(r.Repo, "docker", "docker-compose.cluster.yaml")
			} else {
				composefile = filepath.Join(r.Repo, "docker", "docker-compose.yaml")
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

	// Create a temporary directory as the project directory for the temporary
	// .env file containing the image tag.
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	// Clean up the directory on exit.
	defer os.RemoveAll(dir)

	err = createFile(filepath.Join(dir, ".env"), []byte(fmt.Sprintf("IMAGE_TAG=%s", imageTag)))
	if err != nil {
		return fmt.Errorf("create .env: %w", err)
	}

	// Build a temporary image..
	c := exec.Command(
		"docker",
		"build",
		"--file", dockerfile,
		"--tag", imageTag,
		path,
	)

	c.Stdout = stdout
	c.Stderr = stderr

	err = c.Run()
	if err != nil {
		return fmt.Errorf("build image: %w", err)
	}
	// Remove the built image on exit to prevent
	// TODO: consider making this optional.. should rely on git hash instead
	// of random uid.
	defer exec.Command(
		"docker",
		"rmi",
		imageTag,
	).Run()

	// Best effort to bring containers down..
	defer exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", dir,
		"--file", composefile,
		"down",
		"--remove-orphans",
		"--timeout", "3",
	).Run()

	// Run the app container.
	cmd := exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", dir,
		"--file", composefile,
		"run",
		"--no-TTY",
		"--rm",
		"app",
	)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}
