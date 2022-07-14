package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
)

var (
	generatedHeader = regexp.MustCompile(`^#gobyexample-generated$`)
)

const (
	defaultGoDockerfile = `#gobyexample-generated
# Image to build the Go binary.
FROM golang:1.18-alpine AS build

WORKDIR /opt/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./
RUN go build -v -o /app ./...

# Copy binary to small image for distribution.
FROM alpine

COPY --from=build /app /app

ENTRYPOINT ["/app"]
`

	defaultShellDockerfile = `#gobyexample-generated
FROM natsio/nats-box:0.12.0

WORKDIR /opt/app

COPY . ./
RUN chmod +x main.sh

ENTRYPOINT ["/opt/app/main.sh"]
`
)

var (
	langDockerfiles = map[string]string{
		Go:    defaultGoDockerfile,
		Shell: defaultShellDockerfile,
		CLI:   defaultShellDockerfile,
	}
)

const (
	singleServerComposefile = `#gobyexample-generated
version: '3.9'
services:
  nats:
		image: docker.io/nats:2.8.4
		command:
			- "--debug"
			- "--http_port=8222"
			- "--js"
		ports:
			- "14222:4222"
			- "18222:8222"

	app:
		build: .
		environment:
			- NATS_URL=nats:4222
		depends_on:
			- nats
`

	clusteredServerComposeFile = `#gobyexample-generated
version: '3.9'
services:
  nats1:
    image: docker.io/nats:2.8.4
    command:
		  - "--debug"
      - "--name=nats1"
      - "--cluster_name=c1"
      - "--cluster=nats://nats1:6222"
      - "--routes=nats-route://nats1:6222,nats-route://nats2:6222,nats-route://nats3:6222"
			- "--http_port=8222"
      - "--js"
		ports:
			- "14222:4222"
			- "18222:8222"

  nats2:
    image: docker.io/nats:2.8.4
    command:
		  - "--debug"
      - "--name=nats2"
      - "--cluster_name=c1"
      - "--cluster=nats://nats2:6222"
      - "--routes=nats-route://nats1:6222,nats-route://nats2:6222,nats-route://nats3:6222"
			- "--http_port=8222"
      - "--js"
		ports:
			- "24222:4222"
			- "28222:8222"

  nats3:
    image: docker.io/nats:2.8.4
    command:
		  - "--debug"
      - "--name=nats3"
      - "--cluster_name=c1"
      - "--cluster=nats://nats3:6222"
      - "--routes=nats-route://nats1:6222,nats-route://nats2:6222,nats-route://nats3:6222"
			- "--http_port=8222"
      - "--js"
		ports:
			- "34222:4222"
			- "38222:8222"

	app:
		build: .
		environment:
			- NATS_URL=nats1:4222,nats2:4222,nats3:4222
		depends_on:
			- nats1
			- nats2
			- nats3
`
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

func createDockerfile(lang, dir string, recreate bool) error {
	p := filepath.Join(dir, "Dockerfile")

	f, err := os.Open(p)
	// Exists..
	if err == nil {
		// Regardless if it was generated or manually created.
		if !recreate {
			return nil
		}

		// Check if the file was generated and skip if not..
		r := bufio.NewReader(f)
		l, _, err := r.ReadLine()
		f.Close()
		if err != nil {
			return fmt.Errorf("read line: %w", err)
		}
		if !generatedHeader.Match(l) {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("open file: %w", err)
	}

	// Create new or recreate.
	dcontents, ok := langDockerfiles[lang]
	if !ok {
		return fmt.Errorf("no default Dockerfile for %q", lang)
	}
	contents := bytes.Replace([]byte(dcontents), []byte{'\t'}, []byte("  "), -1)
	err = createFile(p, contents)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	return nil
}

func createComposefile(lang, dir string, recreate bool) error {
	p := filepath.Join(dir, "docker-compose.yaml")

	f, err := os.Open(p)
	// Exists..
	if err == nil {
		// Regardless if it was generated or manually created.
		if !recreate {
			return nil
		}

		// Check if the file was generated and skip if not..
		r := bufio.NewReader(f)
		l, _, err := r.ReadLine()
		f.Close()

		if err != nil {
			return fmt.Errorf("read line: %w", err)
		}
		if !generatedHeader.Match(l) {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("open file: %w", err)
	}

	contents := []byte(singleServerComposefile)
	contents = bytes.Replace(contents, []byte{'\t'}, []byte("  "), -1)
	// Create new or recreate.
	err = createFile(p, contents)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	return nil
}

func generateOutput(dir string, recreate bool) error {
	p := filepath.Join(dir, "output.txt")

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

	output, err := runDockerCompose(dir)
	if err != nil {
		return fmt.Errorf("run docker compose: %s", err)
	}

	// Create new or recreate.
	err = createFile(p, output)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	return nil
}

func runDockerCompose(path string) ([]byte, error) {
	uid := uuid.New().String()[:8]

	err := exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", path,
		"build",
		"app",
	).Run()
	if err != nil {
		return nil, fmt.Errorf("build container: %w", err)
	}

	cmd := exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", path,
		"run",
		"--no-TTY",
		"--rm",
		"--quiet-pull",
		"app",
	)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Best effort to bring containers down..
	defer exec.Command(
		"docker",
		"compose",
		"--project-name", uid,
		"--project-directory", path,
		"down",
		"--remove-orphans",
		"--timeout", "3",
	).Run()

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("run container: %w:\n%s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}
