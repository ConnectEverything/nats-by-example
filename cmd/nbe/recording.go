package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

		tempFile, _ := os.CreateTemp("", "")
		tempName := tempFile.Name()
		tempFile.Close()
		defer os.Remove(tempName)

		// Generate the recording using the pre-built image.
		c := exec.Command(
			"asciinema", "rec",
			"--overwrite",
			"--command", fmt.Sprintf("nbe run --no-ansi=true --quiet --image=%s %s", image, name),
			"--title", fmt.Sprintf("NATS by Example: %s", name),
			tempName,
		)

		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		err = c.Run()
		if err != nil {
			return fmt.Errorf("asciinema rec: %w", err)
		}

		contents, err := os.ReadFile(tempName)
		if err != nil {
			return err
		}

		contents = removeComposeLines(contents)

		os.WriteFile(castFile, contents, 0644)
	}

	c := exec.Command(
		"asciinema", "cat",
		castFile,
	)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("asciinema cat: %w\n%s", err, string(output))
	}

	return os.WriteFile(outputFile, output, 0644)
}

func removeComposeLines(output []byte) []byte {
	re := regexp.MustCompile(`(Network|Container)\s+[^\s]+\s+(Creating|Created|Starting|Started)`)

	buf := bytes.NewBuffer(nil)
	sc := bufio.NewScanner(bytes.NewReader(output))

	for sc.Scan() {
		line := sc.Bytes()

		if re.Find(line) == nil {
			buf.Write(line)
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes()
}
