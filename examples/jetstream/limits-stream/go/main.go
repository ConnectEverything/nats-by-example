package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	// Use the env varibale if running in the container, otherwise use the default.
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Create an unauthenticated connection to NATS.
	nc, _ := nats.Connect(url)

	// Drain is a safe way to to ensure all buffered messages that were published
	// are sent and all buffered messages received on a subscription are processed
	// being closing the connection.
	defer nc.Drain()

	// Access the JetStreamContext which provides methods to create
	// streams and consumers as well as convenience methods for publishing
	// to streams and implicitly creating consumers through `*Subscribe*`
	// methods (more on this in other examples).
	js, _ := nc.JetStream()

	// To get started with JetStream, a stream must be created. The mental
	// model for a stream is that is binds a set of subjects for which
	// messages published to those subjects will be appended (and persisted)
	// to the stream. We will declare the initial stream configuration by
	// specifying the name and subjects.
	cfg := nats.StreamConfig{
		// Stream names are commonly uppercased to visually differentiate
		// them from subjects, but this is not required.
		Name: "EVENTS",
		// Notice this is a slice of subjects, so there can be multiple
		// concrete subjects or ones with wildcards all bound to the same
		// stream.
		Subjects: []string{"events.>"},
	}

	// JetStream provides both file and in-memory storage options. For
	// durability of the stream data, file storage must be chosen to
	// survive crashes and restarts. This is the default for the stream,
	// but we can still set it explicitly.
	cfg.Storage = nats.FileStorage

	// Every stream has what is called a **retention policy**. The default
	// policy is **limits-based** and applies to *all* streams.
	// The standard limits include the maximum number of messages in
	// the stream, the maximum total size in bytes of the stream, and
	// the maximum age of a message. For now, we will not set any limits
	// and to show the behavior applying these limits incrementally.
	js.AddStream(&cfg)
	log.Println("created the stream")

	// Let's publish a few messages which are received by the stream since
	// they match the subject bound to the stream. The `js.Publish` method
	// is a convenience for sending a `nc.Request` and waiting for the
	// acknowledgement.
	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.input_focused", nil)
	log.Println("published 6 messages")

	// There is also is an async form in which the client batches the
	// the messages to the server and then asynchronously receives the
	// the acknowledgements.
	js.PublishAsync("events.input_changed", nil)
	js.PublishAsync("events.input_blurred", nil)
	js.PublishAsync("events.key_pressed", nil)
	js.PublishAsync("events.input_focused", nil)
	js.PublishAsync("events.input_changed", nil)
	js.PublishAsync("events.input_blurred", nil)

	// For a given batch, we select on a channel returned from
	// `js.PublishAsyncComplete`.
	select {
	case <-js.PublishAsyncComplete():
		log.Println("published 6 messages")
	case <-time.After(time.Second):
		log.Fatal("publish took too long")
	}

	// Checking out the stream info, we can see how many messages we
	// have.
	logStreamState(js, cfg.Name)

	// Stream configuration can be dynamically changed. For example,
	// we can set the max messages limit to 10 and it will truncate the
	// two events we had received.

	cfg.MaxMsgs = 10
	js.UpdateStream(&cfg)
	log.Println("set max messages to 10")

	// Checking out the info, we see there are now 10 messages and the
	// first sequence and timestamp are based on the third message.
	logStreamState(js, cfg.Name)

	// Limits can be combined and whichever one is reached, it will
	// be applied to truncate the stream. For example, let's add the
	// max bytes.
	cfg.MaxBytes = 300
	js.UpdateStream(&cfg)
	log.Println("set max bytes to 300")

	// Inspecting the stream info we now see more messages have been
	// truncated to ensure the size is not exceeded.
	logStreamState(js, cfg.Name)

	// Finally, for the last primary limit, we can set the max age.
	cfg.MaxAge = time.Second
	js.UpdateStream(&cfg)
	log.Println("set max age to one second")

	// Looking at the strem info, we still see all the messages..
	logStreamState(js, cfg.Name)

	// Until a second passes...
	time.Sleep(time.Second)

	logStreamState(js, cfg.Name)
}

// Helper function to log the stream state info.
func logStreamState(js nats.JetStreamContext, name string) {
	info, _ := js.StreamInfo(name)
	b, _ := json.MarshalIndent(info.State, "", " ")
	log.Println("inspecting stream info")
	log.Println(string(b))
}
