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

	// Create `JetStream` to use the NATS JetStream API.
	// It allows creating and managing streams and consumers as well as
	// publishing to streams and consuming messages from streams.
	js, _ := jetstream.New(nc)

	// ### Creating the stream
	// Define the stream configuration, specifying `InterestPolicy` for retention, and
	// create the stream.
	cfg := jetstream.StreamConfig{
		Name:      "EVENTS",
		Retention: jetstream.InterestPolicy,
		Subjects:  []string{"events.>"},
	}

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, _ := js.CreateStream(ctx, cfg)
	fmt.Println("created the stream")

	// To demonstrate the base case behavior of the stream without any consumers, we
	// will publish a few messages to the stream.
	js.Publish(ctx, "events.page_loaded", nil)
	js.Publish(ctx, "events.mouse_clicked", nil)
	ack, _ := js.Publish(ctx, "events.input_focused", nil)
	fmt.Println("published 3 messages")

	// We confirm that all three messages were published and the last message sequence
	// is 3.
	fmt.Printf("last message seq: %d\n", ack.Sequence)

	// Checking out the stream info, notice how zero messages are present in
	// the stream, but the `last_seq` is 3 which matches the last ack'ed
	// publish sequence above. Also notice that the `first_seq` is one greater
	// which behaves as a sentinel value indicating the stream is empty. This
	// sequence has not been assigned to a message yet, but can be interpreted
	// as _no messages available_ in this context.
	fmt.Println("# Stream info without any consumers")
	printStreamState(ctx, stream)

	// ### Adding a consumer
	// Now let's add a pull consumer and publish a few
	// more messages. Also note that we are _only_ creating the consumer and
	// have not yet started consuming the messages. This is only to point out
	// that it is not _required_ to be actively consuming messages to show
	// _interest_, but it is the presence of a consumer which the stream cares
	// about to determine retention of messages. [pull]:
	// /examples/jetstream/pull-consumer/go
	cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   "processor-1",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	js.Publish(ctx, "events.mouse_clicked", nil)
	js.Publish(ctx, "events.input_focused", nil)

	// If we inspect the stream info again, we will notice a few differences.
	// It shows two messages (which we expect) and the first and last sequences
	// corresponding to the two messages we just published. We also see that
	// the `consumer_count` is now one.
	fmt.Println("\n# Stream info with one consumer")
	printStreamState(ctx, stream)

	// Now that the consumer is there and showing _interest_ in the messages, we know they
	// will remain until we process the messages. Let's fetch the two messages and ack them.
	msgs, _ := cons.Fetch(2)
	for msg := range msgs.Messages() {
		msg.DoubleAck(ctx)
	}

	// What do we expect in the stream? No messages and the `first_seq` has been set to
	// the _next_ sequence number like in the base case.
	// ☝️ As a quick aside on that second ack, We are using `AckSync` here for this
	// example to ensure the stream state has been synced up for this subsequent
	// retrieval.
	fmt.Println("\n# Stream info with one consumer and acked messages")
	printStreamState(ctx, stream)

	// ### Two or more consumers
	// Since each consumer represents a separate _view_ over a stream, we would expect
	// that if messages were processed by one consumer, but not the other, the messages
	// would be retained. This is indeed the case.
	cons2, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   "processor-2",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	js.Publish(ctx, "events.input_focused", nil)
	js.Publish(ctx, "events.mouse_clicked", nil)

	// Here we fetch 2 messages for `processor-2`. There are two observations to
	// make here. First the fetched messages are the latest two messages that
	// were published just above and not any prior messages since these were
	// already deleted from the stream. This should be apparent now, but this
	// reinforces that a _late_ consumer cannot retroactively show interest. The
	// second point is that the stream info shows that the latest two messages
	// are still present in the stream. This is also expected since the first
	// consumer had not yet processed them.
	msgs, _ = cons2.Fetch(2)
	var msgsMeta []*jetstream.MsgMetadata
	for msg := range msgs.Messages() {
		msg.DoubleAck(ctx)
		meta, _ := msg.Metadata()
		msgsMeta = append(msgsMeta, meta)
	}

	fmt.Printf("msg seqs %d and %d", msgsMeta[0].Sequence.Stream, msgsMeta[1].Sequence.Stream)

	fmt.Println("\n# Stream info with two consumers, but only one set of acked messages")
	printStreamState(ctx, stream)

	// Fetching and ack'ing from the first consumer subscription will result in the messages
	// being deleted.
	msgs, _ = cons.Fetch(2)
	for msg := range msgs.Messages() {
		msg.DoubleAck(ctx)
	}

	fmt.Println("\n# Stream info with two consumers having both acked")
	printStreamState(ctx, stream)

	// A final callout is that _interest_ respects the `FilterSubject` on a consumer.
	// For example, if a consumer defines a filter only for `events.mouse_clicked` events
	// then it won't be considered _interested_ in events such as `events.input_focused`.
	stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "processor-3",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "events.mouse_clicked",
	})

	js.Publish(ctx, "events.input_focused", nil)

	// Fetch and term (also works) and ack from the first consumers that _do_ have interest.
	msgs, _ = cons.Fetch(1)
	msg := <-msgs.Messages()
	msg.Term()
	msgs, _ = cons2.Fetch(1)
	msg = <-msgs.Messages()
	msg.DoubleAck(ctx)

	fmt.Println("\n# Stream info with three consumers with interest from two")
	printStreamState(ctx, stream)
}

// This is just a helper function to print the stream state info 😉.
func printStreamState(ctx context.Context, stream jetstream.Stream) {
	info, _ := stream.Info(ctx)
	b, _ := json.MarshalIndent(info.State, "", " ")
	fmt.Println(string(b))
}
