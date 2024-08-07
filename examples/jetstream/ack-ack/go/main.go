package main

import (
	"context"
	"fmt"
	"log"
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
	// are options to replicate messages in various ways, but that will
	// be explained in later examples.
	cfg := jetstream.StreamConfig{
		Name:     "DOUBLE_ACK_STREAM",
		Subjects: []string{"doubleAckSubject"},
		Storage:  jetstream.MemoryStorage,
	}

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete the stream, so we always have a fresh start for the example
	// don't care if this, errors in this example, it will if the stream exists.
	_ = js.DeleteStream(ctx, "DOUBLE_ACK_STREAM")

	// Let's add/create the stream with the default (no) limits.
	stream, _ := js.CreateStream(ctx, cfg)

	// Publish a couple messages, so we can look at the state. In a real system we might ensure there is no error.
	_, err := js.Publish(ctx, "doubleAckSubject", []byte("A"))
	if err != nil {
		log.Fatal(err)
	}
	_, err = js.Publish(ctx, "doubleAckSubject", []byte("B"))
	if err != nil {
		log.Fatal(err)
	}

	// Consume a message with 2 different consumers
	// The first consumer will Ack without confirmation
	// The second consumer will AckSync which confirms that ack was handled.
	cons1, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "Consumer1",
		FilterSubject: "doubleAckSubject",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Fatal(err)
	}

	cons2, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "Consumer2",
		FilterSubject: "doubleAckSubject",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Consumer 1 will use Ack()
	ci, err := cons1.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Consumer 1")
	fmt.Printf("  Start\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Get one message with Consumer Next()
	m, err := cons1.Next()
	if err != nil {
		log.Fatal(err)
	}
	ci, err = cons1.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  After received but before ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Ack the message.
	err = m.Ack()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  After ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Consumer 2 will use DoubleAck()
	ci, err = cons2.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Consumer 2")
	fmt.Printf("  Start\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Get one message with Consumer Next()
	m, err = cons2.Next()
	if err != nil {
		log.Fatal(err)
	}
	ci, err = cons2.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  After received but before ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// DoubleAck the message.
	// The thing about DoubleAck is it is a request reply.
	// Make a request to the server, the server does work, the server replies.
	// It's rare, but the request could get to the server,
	// the server handles it and sends the response, but it
	// does not make it back to the client in time. This is where
	// knowledge of the environment and network is important
	// when setting connection and request time-outs.
	err = m.DoubleAck(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  After ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)
}
