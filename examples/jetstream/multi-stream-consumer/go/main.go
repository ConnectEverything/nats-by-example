package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, _ := nats.Connect(natsURL)
	js, _ := nc.JetStream()

	// Create a stream for each region.
	js.AddStream(&nats.StreamConfig{
		Name:     "EVENTS-EU",
		Subjects: []string{"events.eu.>"},
	})

	js.AddStream(&nats.StreamConfig{
		Name:     "EVENTS-US",
		Subjects: []string{"events.us.>"},
	})

	// Create a consumer for each stream. Both publish to the same deliver subject.
	// This is a straightforward way to do this in a single account. It is recommended
	// a user is created with specific permissions to subscribe to this subject.
	js.AddConsumer("EVENTS-EU", &nats.ConsumerConfig{
		Durable:        "processor",
		DeliverSubject: "push.events",
		AckPolicy:      nats.AckExplicitPolicy,
	})

	js.AddConsumer("EVENTS-US", &nats.ConsumerConfig{
		Durable:        "processor",
		DeliverSubject: "push.events",
		AckPolicy:      nats.AckExplicitPolicy,
	})

	// Publish messages to each stream.
	js.Publish("events.eu.page_loaded", nil)
	js.Publish("events.eu.input_focused", nil)
	js.Publish("events.us.page_loaded", nil)
	js.Publish("events.us.mouse_clicked", nil)
	js.Publish("events.eu.mouse_clicked", nil)
	js.Publish("events.us.input_focused", nil)

	// Subscribe to the deliver subject with core NATS subscription. Observe that
	// messages from both streams are being received and can be ack'ed.
	sub, _ := nc.SubscribeSync("push.events")
	defer sub.Drain()

	for {
		msg, err := sub.NextMsg(time.Second)
		if err == nats.ErrTimeout {
			break
		}

		fmt.Println(msg.Subject)
		msg.Ack()
	}

	// Confirm the consumer state is updated.
	info1, _ := js.ConsumerInfo("EVENTS-EU", "processor")
	fmt.Printf("eu: last delivered: %d, num pending: %d\n", info1.Delivered.Stream, info1.NumPending)
	info2, _ := js.ConsumerInfo("EVENTS-US", "processor")
	fmt.Printf("us: last delivered: %d, num pending: %d\n", info2.Delivered.Stream, info2.NumPending)
}
