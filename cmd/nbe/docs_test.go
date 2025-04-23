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
	for _, test := range []struct {
		name     string
		input    string
		expected string
		delim    string
		open     string
		close    string
	}{
		{
			name: "cStyle",
			input: `
				/*
				Hello world
				This is a comment
				with some indents
				and more //
				*/
`,
			expected: `Hello world
This is a comment
with some indents
and more //`,
			open:  "/*",
			close: "*/",
		},
		{
			name: "websocket",
			input: `
				<!--
				Hello world
				This is a comment
				with some indents
				and more //
				-->
`,
			expected: `Hello world
This is a comment
with some indents
and more //`,
			open:  "<!--",
			close: "-->",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := strings.Split(test.input, "\n")
			output, prefix := cleanMultiCommentLines(lines, test.open, test.close)

			t.Logf("%v", []byte(prefix))

			if diff := cmp.Diff(test.expected, output); diff != "" {
				t.Error(diff)
			}
		})
	}
}
