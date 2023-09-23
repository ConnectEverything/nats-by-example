package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
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
	defer nc.Drain()

	// Access `JetStream` to use the JS APIs.
	js, _ := jetstream.New(nc)

	// ### Creating the stream
	// Define the stream configuration, specifying `WorkQueuePolicy` for
	// retention, and create the stream.
	cfg := jetstream.StreamConfig{
		Name:      "EVENTS",
		Retention: jetstream.WorkQueuePolicy,
		Subjects:  []string{"events.>"},
	}

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, _ := js.CreateStream(ctx, cfg)
	fmt.Println("created the stream")

	// ### Queue messages
	// Publish a few messages.
	js.Publish(ctx, "events.us.page_loaded", nil)
	js.Publish(ctx, "events.eu.mouse_clicked", nil)
	js.Publish(ctx, "events.us.input_focused", nil)
	fmt.Println("published 3 messages")

	// Checking the stream info, we see three messages have been queued.
	fmt.Println("# Stream info without any consumers")
	printStreamState(ctx, stream)

	// ### Adding a consumer
	// Now let's add a consumer and publish a few more messages.
	// [pull]: /examples/jetstream/pull-consumer/go
	cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name: "processor-1",
	})

	// Fetch and ack the queued messages.
	msgs, _ := cons.Fetch(3)
	for msg := range msgs.Messages() {
		msg.DoubleAck(ctx)
	}

	// Checking the stream info again, we will notice no messages
	// are available.
	fmt.Println("\n# Stream info with one consumer")
	printStreamState(ctx, stream)

	// ### Exclusive non-filtered consumer
	// As noted in the description above, work-queue streams can only have
	// at most one consumer with interest on a subject at any given time.
	// Since the pull consumer above is not filtered, if we try to create
	// another one, it will fail.
	_, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name: "processor-2",
	})
	fmt.Println("\n# Create an overlapping consumer")
	fmt.Println(err)

	// However if we delete the first one, we can then add the new one.
	stream.DeleteConsumer(ctx, "processor-1")

	_, err = stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name: "processor-2",
	})
	fmt.Printf("created the new consumer? %v\n", err == nil)
	stream.DeleteConsumer(ctx, "processor-2")

	// ### Multiple filtered consumers
	// To create multiple consumers, a subject filter needs to be applied.
	// For this example, we could scope each consumer to the geo that the
	// event was published from, in this case `us` or `eu`.
	fmt.Println("\n# Create non-overlapping consumers")
	cons1, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          "processor-us",
		FilterSubject: "events.us.>",
	})
	cons2, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          "processor-eu",
		FilterSubject: "events.eu.>",
	})

	js.Publish(ctx, "events.eu.mouse_clicked", nil)
	js.Publish(ctx, "events.us.page_loaded", nil)
	js.Publish(ctx, "events.us.input_focused", nil)
	js.Publish(ctx, "events.eu.page_loaded", nil)
	fmt.Println("published 4 messages")

	msgs, _ = cons1.Fetch(2)
	for msg := range msgs.Messages() {
		fmt.Printf("us sub got: %s\n", msg.Subject())
		msg.Ack()
	}

	msgs, _ = cons2.Fetch(2)
	for msg := range msgs.Messages() {
		fmt.Printf("eu sub got: %s\n", msg.Subject())
		msg.Ack()
	}
}

// This is just a helper function to print the stream state info 😉.
func printStreamState(ctx context.Context, stream jetstream.Stream) {
	info, _ := stream.Info(ctx)
	b, _ := json.MarshalIndent(info.State, "", " ")
	fmt.Println(string(b))
}
