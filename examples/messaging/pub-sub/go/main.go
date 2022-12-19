package main

import (
	"fmt"
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

	// Drain is a safe way to to ensure all buffered messages that were published
	// are sent and all buffered messages received on a subscription are processed
	// being closing the connection.
	defer nc.Drain()

	// Messages are published to subjects. Although there are no subscribers,
	// this will be published successfully.
	nc.Publish("greet.joe", []byte("hello"))

	// Let's create a subscription on the greet.* wildcard.
	sub, _ := nc.SubscribeSync("greet.*")

	// For a synchronous subscription, we need to fetch the next message.
	// However.. since the publish occured before the subscription was
	// established, this is going to timeout.
	msg, _ := sub.NextMsg(10 * time.Millisecond)
	fmt.Println("subscribed after a publish...")
	fmt.Printf("msg is nil? %v\n", msg == nil)

	// Publish a couple messages.
	nc.Publish("greet.joe", []byte("hello"))
	nc.Publish("greet.pam", []byte("hello"))

	// Since the subscription is established, the published messages will
	// immediately be broadcasted to all subscriptions. They will land in
	// their buffer for subsequent NextMsg calls.
	msg, _ = sub.NextMsg(10 * time.Millisecond)
	fmt.Printf("msg data: %q on subject %q\n", string(msg.Data), msg.Subject)

	msg, _ = sub.NextMsg(10 * time.Millisecond)
	fmt.Printf("msg data: %q on subject %q\n", string(msg.Data), msg.Subject)

	// One more for good measures..
	nc.Publish("greet.bob", []byte("hello"))

	msg, _ = sub.NextMsg(10 * time.Millisecond)
	fmt.Printf("msg data: %q on subject %q\n", string(msg.Data), msg.Subject)
}
