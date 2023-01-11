package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	// Use the env variable if running in the container, otherwise use the default.
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Create an unauthenticated connection to NATS.
	nc, _ := nats.Connect(url)
	defer nc.Drain()

	js, _ := nc.JetStream()

	partitions := 5
	for i := 0; i < partitions; i++ {
		name := fmt.Sprintf("events-%d", i)
		subject := fmt.Sprintf("events.*.%d", i)

		js.AddStream(&nats.StreamConfig{
			Name:     name,
			Subjects: []string{subject},
		})
	}

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
	fmt.Println("published 6 messages")

	// There is also is an async form in which the client batches the
	// messages to the server and then asynchronously receives the
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
		fmt.Println("published 6 messages")
	case <-time.After(time.Second):
		log.Fatal("publish took too long")
	}

	// Checking out the stream info, we can see how many messages we
	// have.
	printStreamState(js, cfg.Name)

	// Stream configuration can be dynamically changed. For example,
	// we can set the max messages limit to 10 and it will truncate the
	// two initial events in the stream.

	cfg.MaxMsgs = 10
	js.UpdateStream(&cfg)
	fmt.Println("set max messages to 10")

	// Checking out the info, we see there are now 10 messages and the
	// first sequence and timestamp are based on the third message.
	printStreamState(js, cfg.Name)

	// Limits can be combined and whichever one is reached, it will
	// be applied to truncate the stream. For example, let's set a
	// maximum number of bytes for the stream.
	cfg.MaxBytes = 300
	js.UpdateStream(&cfg)
	fmt.Println("set max bytes to 300")

	// Inspecting the stream info we now see more messages have been
	// truncated to ensure the size is not exceeded.
	printStreamState(js, cfg.Name)

	// Finally, for the last primary limit, we can set the max age.
	cfg.MaxAge = time.Second
	js.UpdateStream(&cfg)
	fmt.Println("set max age to one second")

	// Looking at the stream info, we still see all the messages..
	printStreamState(js, cfg.Name)

	// until a second passes.
	fmt.Println("sleeping one second...")
	time.Sleep(time.Second)

	printStreamState(js, cfg.Name)
}

// This is just a helper function to print the stream state info 😉
func printStreamState(js nats.JetStreamContext, name string) {
	info, _ := js.StreamInfo(name)
	b, _ := json.MarshalIndent(info.State, "", " ")
	fmt.Println("inspecting stream info")
	fmt.Println(string(b))
}
