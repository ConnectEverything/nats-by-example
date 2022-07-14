package main

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func checkEqual[T comparable](t *testing.T, a, b T) {
	t.Helper()
	if a != b {
		t.Error("not equal")
	}
}

func logBlocks(t *testing.T, blocks []*Block) {
	b, _ := yaml.Marshal(blocks)
	t.Log(string(b))
}

func TestParseLineType(t *testing.T) {
	// C-style
	checkEqual(t, parseLineType(Go, `  /* hello`), OpenMultiCommentLine)
	checkEqual(t, parseLineType(Go, `yep */`), CloseMultiCommentLine)
	checkEqual(t, parseLineType(Go, `		// ba`), SingleCommentLine)
	checkEqual(t, parseLineType(Go, ` /* meh  */ `), NormalLine)
	checkEqual(t, parseLineType(Go, ` Foo int		// int`), NormalLine)
	checkEqual(t, parseLineType(Go, ` 1 / 2 `), NormalLine)
	checkEqual(t, parseLineType(Go, `			`), EmptyLine)

	// Whitespace-sensitive
	checkEqual(t, parseLineType(Python, `		#  ba`), SingleCommentLine)
	checkEqual(t, parseLineType(Python, `##ba`), SingleCommentLine)
}

func TestParseReader(t *testing.T) {
	goCode := `/*
Package foo provides utilies for interacting with JetStream.

*/
package foo

// Read stream..
func ReadStream(js nats.JetStreamContext, name string) ([]*nats.Msg, error) {
  ...
}

`
	blocks, err := parseReader(Go, bytes.NewBuffer([]byte(goCode)))
	if err != nil {
		t.Fatal(err)
	}
	checkEqual(t, len(blocks), 4)

	pythonCode := `# Package foo
import csv

with open('somefile.txt') as f:
	# Initialize a new CSV reader

	cr := csv.reader(f)

	# Read
	# all
	# the

	# lines
	lines := tuple(cr)

`

	blocks, err = parseReader(Python, bytes.NewBuffer([]byte(pythonCode)))
	if err != nil {
		t.Fatal(err)
	}
	checkEqual(t, len(blocks), 6)
}
