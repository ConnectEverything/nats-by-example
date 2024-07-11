package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	// Use the env variable if running in the container, otherwise use the default.
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Create an unauthenticated connection to NATS.
	nc, _ := nats.Connect(url)

	// Drain is a safe way to ensure all buffered messages that were published
	// are sent and all buffered messages received on a subscription are processed
	// being closing the connection.
	defer nc.Drain()

	// Access `JetStream` which provides methods to create
	// streams and consumers as well as convenience methods for publishing
	// to streams and consuming messages from the streams.
	js, _ := jetstream.New(nc)

	// We will declare the initial stream configuration by specifying
	// the name and subjects. Stream names are commonly uppercase to
	// visually differentiate them from subjects, but this is not required.
	// A stream can bind one or more subjects which almost always include
	// wildcards. In addition, no two streams can have overlapping subjects
	// otherwise the primary messages would be persisted twice. There
	// are option to replicate messages in various ways, but that will
	// be explained in later examples.
	cfg := jetstream.StreamConfig{
		Name:     "SUBJECTS",
		Subjects: []string{"plain", "greater.>", "star.*"},
		Storage:  jetstream.MemoryStorage,
	}

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete the stream, so we always have a fresh start for the example
	// don't care if this, errors in this example, it will if the stream exists.
	_ = js.DeleteStream(ctx, "SUBJECTS")

	// Let's add/create the stream with the default (no) limits.
	stream, _ := js.CreateStream(ctx, cfg)

	// Publish a message. In a real system we might ensure there is no error.
	js.Publish(ctx, "plain", []byte("plain-data"))

	// Stream Info contains State, which contains a map, Subjects, of subjects to their count
	// but the server only collects and returns that information if
	// WithSubjectFilter is set with a non-empty subject filter.
	// To get all subjects, set the filter to &gt;
	si, _ := stream.Info(ctx, jetstream.WithSubjectFilter(">"))
	fmt.Println("After publishing a message to a subject, it appears in state.")
	for k, v := range si.State.Subjects {
		fmt.Printf("  subject '%s' has %v message(s)\n", k, v)
	}

	// Publish some more messages, this time against wildcard subjects
	js.Publish(ctx, "greater.A", []byte("gtA-1"))
	js.Publish(ctx, "greater.A", []byte("gtA-2"))
	js.Publish(ctx, "greater.A.B", []byte("gtAB-1"))
	js.Publish(ctx, "greater.A.B", []byte("gtAB-2"))
	js.Publish(ctx, "greater.A.B.C", []byte("gtABC"))
	js.Publish(ctx, "greater.B.B.B", []byte("gtBBB"))
	js.Publish(ctx, "star.1", []byte("star1-1"))
	js.Publish(ctx, "star.1", []byte("star1-2"))
	js.Publish(ctx, "star.2", []byte("star2"))

	si, _ = stream.Info(ctx, jetstream.WithSubjectFilter(">"))
	fmt.Println("Wildcard subjects show the actual subject, not the template.")
	for k, v := range si.State.Subjects {
		fmt.Printf("  subject '%s' has %v message(s)\n", k, v)
	}

	// ### Subject Filtering
	// Instead of &gt;, you can filter for a specific subject
	si, _ = stream.Info(ctx, jetstream.WithSubjectFilter("greater.>"))
	fmt.Println("Filtering the subject returns only matching entries ['greater.>']")
	for k, v := range si.State.Subjects {
		fmt.Printf("  subject '%s' has %v message(s)\n", k, v)
	}

	si, _ = stream.Info(ctx, jetstream.WithSubjectFilter("greater.A.>"))
	fmt.Println("Filtering the subject returns only matching entries ['greater.A.>']")
	for k, v := range si.State.Subjects {
		fmt.Printf("  subject '%s' has %v message(s)\n", k, v)
	}
}
