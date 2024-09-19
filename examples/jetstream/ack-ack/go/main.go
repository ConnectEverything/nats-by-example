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

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a stream.
	// Any stream can work with confirmed acks.
	stream, _ := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "STREAM",
		Subjects: []string{"data"},
		Storage:  jetstream.MemoryStorage,
	})

	// Publish a couple messages.
	js.Publish(ctx, "data", []byte("A"))
	js.Publish(ctx, "data", []byte("B"))

	// Create a consumer with explicit ack policy.
	consumer, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          "CONSUMER",
		FilterSubject: "data",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})

	ci, _ := consumer.Info(ctx)

	fmt.Printf("  Start\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Get the next pending message for the consumer.
	m, _ := consumer.Next()

	// Fetch the state of the consumer.
	ci, _ = consumer.Info(ctx)
	fmt.Printf("  After received but before ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Ack the message.
	// We're using standard ack here, which does not wait for the server to confirm it received it.
	// If server is experiencing high load, or the connection is severed before the ack is received, the ack can be lost, leading to a redelivery.
	err := m.Ack()
	// Error here only means that the client failed to send the ack message.
	// There will be no error if the client published the ack, but the server did not receive (or process) it.
	if err != nil {
		log.Fatal(err)
	}

	ci, _ = consumer.Info(ctx)
	fmt.Printf("  After ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)

	// Get the next pending message for the consumer.
	m, _ = consumer.Next()

	// Fetch the state of the consumer.
	ci, _ = consumer.Info(ctx)
	fmt.Printf("  After received but before ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)
	// Here we use confirmed ack.
	// This one will wait for the server to confirm that the ack was received and processed.
	// It allows us to react to the ack being lost by sending it again, potentially avoiding redelivery.
	err = m.DoubleAck(ctx)
	// This error can mean that the ack was failed to be send, or that the server failed to confirm it.
	if err != nil {
		log.Fatal(err)
	}

	ci, _ = consumer.Info(ctx)
	fmt.Printf("  After ack\n    # pending messages: %d\n    # messages with ack pending: %d\n", ci.NumPending, ci.NumAckPending)
}
