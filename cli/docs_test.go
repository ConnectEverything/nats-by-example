package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCleanCodeLines(t *testing.T) {
	input := `
    nc, _ := nats.Connect(nats.DefaultURL)
    nc.Subscribe("foo.>", func(msg *nats.Msg) {
      switch msg.Subject {
      case "foo.bar":
        msg.Respond([]byte("bar"))
      case "foo.baz":
        msg.Respond([]byte("baz"))
      }
    })
`

	expected := `nc, _ := nats.Connect(nats.DefaultURL)
nc.Subscribe("foo.>", func(msg *nats.Msg) {
  switch msg.Subject {
  case "foo.bar":
    msg.Respond([]byte("bar"))
  case "foo.baz":
    msg.Respond([]byte("baz"))
  }
})`

	lines := strings.Split(input, "\n")
	output := cleanCodeLines(lines)

	if diff := cmp.Diff(expected, output); diff != "" {
		t.Error(diff)
	}
}

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
	output := cleanSingleCommentLines(lines)

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
	output := cleanMultiCommentLines(lines)

	if diff := cmp.Diff(expected, output); diff != "" {
		t.Error(diff)
	}
}
