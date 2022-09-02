package main

import (
	"encoding/json"
	"fmt"
	"os"

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

	// Access `JetStreamContext` to use the JS APIs.
	js, _ := nc.JetStream()

	// ### Creating the stream
	// Define the stream configuration, specifying `InterestPolicy` for retention, and
	// create the stream.
	cfg := &nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.InterestPolicy,
		Subjects:  []string{"events.>"},
	}

	js.AddStream(cfg)
	fmt.Println("created the stream")

	// To demonstrate the base case behavior of the stream without any consumers, we
	// will publish a few messages to the stream.
	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	ack, _ := js.Publish("events.input_focused", nil)
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
	printStreamState(js, cfg.Name)

	// ### Adding a consumer
	// Now let's add a consumer and publish a few more messages. It can be a [push][push]
	// or [pull][pull] consumer. For this example, we are defining a pull consumer since
	// we aren't specifying a `DeliverSubject`. Also note that we are _only_ creating the
	// consumer and have not yet bound a subscription to actually receive messages. This
	// is only to point out that a subscription is not _required_ to show _interest_, but
	// it is the presence of a consumer which the stream cares about to determine retention
	// of messages.
	// [push]: /examples/jetstream/push-consumer/go
	// [pull]: /examples/jetstream/pull-consumer/go
	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:   "processor-1",
		AckPolicy: nats.AckExplicitPolicy,
	})

	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.input_focused", nil)

	// If we inspect the stream info again, we will notice a few differences.
	// It shows two messages (which we expect) and the first and last sequences
	// corresponding to the two messages we just published. We also see that
	// the `consumer_count` is now one.
	fmt.Println("\n# Stream info with one consumer")
	printStreamState(js, cfg.Name)

	// Now that the consumer is there and showing _interest_ in the messages, we know they
	// will remain until we process the messages. Let's create a subscription bound
	// to the pull consumer, fetch the two messages and ack them.
	sub1, _ := js.PullSubscribe("", "processor-1", nats.Bind(cfg.Name, "processor-1"))
	defer sub1.Unsubscribe()

	msgs, _ := sub1.Fetch(2)
	msgs[0].Ack()
	msgs[1].AckSync()

	// What do we expect in the stream? No messages and the `first_seq` has been set to
	// the _next_ sequence number like in the base case.
	// ☝️ As a quick aside on that second ack, We are using `AckSync` here for this
	// example to ensure the stream state has been synced up for this subsequent
	// retrieval.
	fmt.Println("\n# Stream info with one consumer and acked messages")
	printStreamState(js, cfg.Name)

	// ### Two or more consumers
	// Since each consumer represents a separate _view_ over a stream, we would expect
	// that if messages were processed by one consumer, but not the other, the messages
	// would be retained. This is indeed the case.
	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:   "processor-2",
		AckPolicy: nats.AckExplicitPolicy,
	})

	js.Publish("events.input_focused", nil)
	js.Publish("events.mouse_clicked", nil)

	// Here we bind a subscription for `processor-2`, followed by a fetch and ack. There are
	// two observations to make here. First the fetched messages are the latest two messages
	// that were published just above and not any prior messages since these were already
	// deleted from the stream. This should be apparent now, but this reinforces that a _late_
	// consumer cannot retroactively show interest.
	// The second point is that the stream info shows that the latest two messages are still
	// present in the stream. This is also expected since the first consumer had not yet
	// processed them.
	sub2, _ := js.PullSubscribe("", "processor-2", nats.Bind(cfg.Name, "processor-2"))
	defer sub2.Unsubscribe()

	msgs, _ = sub2.Fetch(2)
	md0, _ := msgs[0].Metadata()
	md1, _ := msgs[1].Metadata()
	fmt.Printf("msg seqs %d and %d", md0.Sequence.Stream, md1.Sequence.Stream)
	msgs[0].Ack()
	msgs[1].AckSync()

	fmt.Println("\n# Stream info with two consumers, but only one set of acked messages")
	printStreamState(js, cfg.Name)

	// Fetching and ack'ing from the first consumer subscription will result in the messages
	// being deleted.
	msgs, _ = sub1.Fetch(2)
	msgs[0].Ack()
	msgs[1].AckSync()

	fmt.Println("\n# Stream info with two consumers having both acked")
	printStreamState(js, cfg.Name)

	// A final callout is that _interest_ respects the `FilterSubject` on a consumer.
	// For example, if a consumer defines a filter only for `events.mouse_clicked` events
	// then it won't be considered _interested_ in events such as `events.input_focused`.
	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:       "processor-3",
		AckPolicy:     nats.AckExplicitPolicy,
		FilterSubject: "events.mouse_clicked",
	})

	js.Publish("events.input_focused", nil)

	// Fetch and term (also works) and ack from the first consumers that _do_ have interest.
	msgs, _ = sub1.Fetch(1)
	msgs[0].Term()
	msgs, _ = sub2.Fetch(1)
	msgs[0].AckSync()

	fmt.Println("\n# Stream info with three consumers with interest from two")
	printStreamState(js, cfg.Name)
}

// This is just a helper function to print the stream state info 😉.
func printStreamState(js nats.JetStreamContext, name string) {
	info, _ := js.StreamInfo(name)
	b, _ := json.MarshalIndent(info.State, "", " ")
	fmt.Println(string(b))
}
