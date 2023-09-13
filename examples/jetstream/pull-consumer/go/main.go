package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
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

	// Access JetStream for managing streams and consumers as well as for
	// publishing and consuming messages to and from the stream.
	js, _ := jetstream.New(nc)

	streamName := "EVENTS"

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	stream, _ := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{"events.>"},
	})

	// Publish a few messages for the example.
	js.Publish(ctx, "events.1", nil)
	js.Publish(ctx, "events.2", nil)
	js.Publish(ctx, "events.3", nil)

	// Create the consumer bound to the previously created stream. If durable
	// name is not supplied, consumer will be removed after InactiveThreshold
	// (defaults to 5 seconds) is reached when not actively consuming messages.
	// `Name` is optional, if not provided it will be auto-generated.
	// For this example, let's use the consumer with no options, which will
	// be ephemeral with auto-generated name.
	cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{})

	// Messages can be _consumed_  continuously in callback using `Consume`
	// method. `Consume` can be supplied with various options, but for this
	// example we will use the default ones. WaitGroup is used as part of this
	// example to make sure to stop processing  after we process 3 messages (so
	// that it does not interfere with other examples).
	wg := sync.WaitGroup{}
	wg.Add(3)

	cc, _ := cons.Consume(func(msg jetstream.Msg) {
		msg.Ack()
		fmt.Println("received msg on", msg.Subject())
		wg.Done()
	})
	wg.Wait()

	// Consume can be stopped by calling `Stop` on the returned ConsumerContext.
	// This will stop the callback from being called and stop retrieving the
	// messages.
	cc.Stop()

	// Publish more messages.
	js.Publish(ctx, "events.1", nil)
	js.Publish(ctx, "events.2", nil)
	js.Publish(ctx, "events.3", nil)

	// We can _fetch_ messages in batches. The first argument being the
	// batch size which is the _maximum_ number of messages that should
	// be returned. For this first fetch, we ask for two and we will get
	// those since they are in the stream.
	msgs, _ := cons.Fetch(2)
	var i int
	for msg := range msgs.Messages() {
		// Let's ack the messages so they are not redelivered.
		msg.Ack()
		i++
	}
	fmt.Printf("got %d messages\n", i)

	// `Fetch` puts messages on the returned `Messages()` channel. This channel
	// will only be closed when the requested number of messages have been
	// received or the operation times out. If we do not want to wait for the
	// rest of the messages and want to quickly return as many messages as there
	// are available (up to provided batch size), we can use `FetchNoWait`
	// instead.
	// Here, because we have already received two messages, we will only get
	// one more.
	msgs, _ = cons.FetchNoWait(100)
	i = 0
	for msg := range msgs.Messages() {
		msg.Ack()
		i++
	}
	fmt.Printf("got %d messages\n", i)

	// Finally, if we are at the end of the stream and we call fetch,
	// the call will be blocked until the "max wait" time which is 5
	// seconds by default, but this can be set explicitly as an option.
	fetchStart := time.Now()
	msgs, _ = cons.Fetch(1, jetstream.FetchMaxWait(time.Second))
	i = 0
	for msg := range msgs.Messages() {
		msg.Ack()
		i++
	}

	fmt.Printf("got %d messages in %v\n", i, time.Since(fetchStart))

	// Durable consumers can be created by specifying the Durable name.
	// Durable consumers are not removed automatically regardless of the
	// InactiveThreshold. They can be removed by calling `DeleteConsumer`.
	dur, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: "processor",
	})

	// Consume and fetch work the same way for durable consumers.
	msgs, _ = dur.Fetch(1)
	msg := <-msgs.Messages()
	fmt.Printf("received %q from durable consumer\n", msg.Subject())

	// While ephemeral consumers will be removed after InactiveThreshold, durable
	// consumers have to be removed explicitly if no longer needed.
	stream.DeleteConsumer(ctx, "processor")

	// Let's try to get the consumer to make sure it's gone.
	_, err := stream.Consumer(ctx, "processor")

	fmt.Println("consumer deleted:", errors.Is(err, jetstream.ErrConsumerNotFound))
}
