package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func generateRecording(repo, example string, recreate bool) error {
	castFile := filepath.Join(repo, example, "output.cast")
	outputFile := filepath.Join(repo, example, "output.txt")

	_, err := os.Stat(castFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat file: %w", err)
		}
	}

	// Does not exist, or force recreate.
	if err != nil || recreate {
		b := ImageBuilder{
			Repo:    repo,
			Example: example,
			Verbose: true,
		}

		image, err := b.Run()
		if err != nil {
			return fmt.Errorf("build image: %w", err)
		}

		name := strings.TrimPrefix(example, "examples/")

		// Generate the recording using the pre-built image.
		c := exec.Command(
			"asciinema", "rec",
			"--overwrite",
			"--command", fmt.Sprintf("nbe run --no-ansi=true --quiet --image=%s %s", image, name),
			"--title", fmt.Sprintf("NATS by Example: %s", name),
			castFile,
		)

		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		err = c.Run()
		if err != nil {
			return fmt.Errorf("asciinema rec: %w", err)
		}
	}

	// TODO: remove container startup prefix...
	c := exec.Command(
		"asciinema", "cat",
		castFile,
	)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("asciinema cat: %w\n%s", err, string(output))
	}

	return ioutil.WriteFile(outputFile, output, 0644)
}
