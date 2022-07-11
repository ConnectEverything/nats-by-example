package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCleanSingleCommentLines(t *testing.T) {
	input := `
				// Hello world
				// This is a comment
				// with some indents
				// and more //
				//
`

	expected := `Hello world
This is a comment
with some indents
and more //`

	lines := strings.Split(input, "\n")
	output, prefix := cleanSingleCommentLines(lines, "//")

	t.Logf("%v", []byte(prefix))

	if diff := cmp.Diff(expected, output); diff != "" {
		t.Error(diff)
	}
}

func TestCleanMultiCommentLines(t *testing.T) {
	input := `
				/*
				Hello world
				This is a comment
				with some indents
				and more //
				*/
`

	expected := `Hello world
This is a comment
with some indents
and more //`

	lines := strings.Split(input, "\n")
	output, prefix := cleanMultiCommentLines(lines)

	t.Logf("%v", []byte(prefix))

	if diff := cmp.Diff(expected, output); diff != "" {
		t.Error(diff)
	}
}
