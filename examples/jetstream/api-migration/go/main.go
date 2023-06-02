package main

import (
	"context"
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

	// ## Legacy JetStream API
	// First lets look at the legacy API for creating a push consumer
	// and subscribing to it to receive messages.
	fmt.Println("# Legacy API")

	// The legacy JetStream API exposed `JetStream()` method on the
	// nats.Conn itself.
	oldJS, _ := nc.JetStream()

	// Declare a simple stream and populate the stream
	// with a few messages.
	streamName := "EVENTS"

	oldJS.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"events.>"},
	})

	oldJS.Publish("events.1", nil)
	oldJS.Publish("events.2", nil)
	oldJS.Publish("events.3", nil)

	// ### Continuous message retrieval with `Subscribe()`
	// Using legacy API, the only way to continuously receive messages
	// is to use push consumers.
	// The easiest way to create a consumer and start consuming messages
	// using the legacy API is to use the `Subscribe()` method. This
	// method will implicitly create an ephemeral consumer and start
	// delivering messages to the provided `nats.MsgHandler` function.
	// In order to bind to and existing stream, the provided subject
	// must overlap with the subjects defined on the stream.
	fmt.Println("# Subscribe with ephemeral consumer")
	sub, _ := oldJS.Subscribe("events.>", func(msg *nats.Msg) {
		fmt.Printf("received %q\n", msg.Subject)
		msg.Ack()
	}, nats.AckExplicit())
	time.Sleep(100 * time.Millisecond)

	// Unsubscribing this subscription will result in the ephemeral consumer
	// being deleted. Note, even if this is omitted and the process ends
	// or is interrupted, the server will eventually clean-up the ephemeral
	// when it determines the subscription is no longer active.
	sub.Unsubscribe()

	// Alternatively, binding to an existing stream can be done by
	// providing the stream in `nats.BindStream` option. In that case,
	// the subject can be omitted since it is already defined on the
	// stream (or subject can be provided to further filter the result).
	// The equivalent of the above would be:
	sub, _ = oldJS.Subscribe("", func(msg *nats.Msg) {
		fmt.Printf("received %q\n", msg.Subject)
		msg.Ack()
	}, nats.AckExplicit(), nats.BindStream(streamName))
	time.Sleep(100 * time.Millisecond)

	// ### Binding to an existing consumer

	// In order to create a consumer outside of the `Subscribe` method,
	// the `AddConsumer` method can be used. This method will create a
	// new consumer using the provided configuration.
	consumerName := "dur-1"
	oldJS.AddConsumer(streamName, &nats.ConsumerConfig{
		Name:              consumerName,
		DeliverSubject:    "handler-1",
		AckPolicy:         nats.AckExplicitPolicy,
		InactiveThreshold: 10 * time.Minute,
	})

	// ### Retrieving messages synchronously with `SubscribeSync()`
	// In order to retrieve messages synchronously, the `SubscribeSync()`
	// method can be used. This method will create a push consumer
	// and a subscription to receive messages.
	// `SubscribeSync` can also be used to bind to existing streams, consumers
	// etc. (similarly to `Subscribe`).
	fmt.Println("# SubscribeSync")
	sub, _ = oldJS.SubscribeSync("events.>", nats.AckExplicit())

	// To *receive* a message, call `NextMsg` with a timeout. The timeout
	// applies when pending count is zero and the consumer has fully caught
	// up to the available messages in the stream. If no messages become
	// available, this call will only block until the timeout.
	msg, _ := sub.NextMsg(time.Second)
	fmt.Printf("received %q\n", msg.Subject)
	msg.Ack()

	// ### Pull consumers
	// The legacy API also supports pull consumers. However, these are
	// greatly limited in functionality since it is only possible to pull
	// a specific number of messages, without any optimization or prefetching.
	// That makes using pull consumers in the legacy API inefficient in contrast
	// to push consumers.
	// In order to create a pull consumer and consume messages, the
	// `PullSubscribe` method can be used.
	fmt.Println("# Subscribe with pull consumer")
	sub, _ = oldJS.PullSubscribe("events.>", "pull-cons", nats.AckExplicit())

	// Messages can be retrieved using the `Fetch` or `FetchBatch` methods.
	// `Fetch` will retrieve up to the provided number of messages and block
	// until the either all messages are available or the timeout is reached.
	fmt.Println("# Fetch")
	msgs, _ := sub.Fetch(2, nats.MaxWait(100*time.Millisecond))
	for _, msg := range msgs {
		fmt.Printf("received %q\n", msg.Subject)
		msg.Ack()
	}

	// `FetchBatch` is similar to `Fetch` except it will return a channel
	// on which the messages will be delivered. This allows for processing
	// messages as they become available.
	fmt.Println("# FetchBatch")
	msgs2, _ := sub.FetchBatch(2, nats.MaxWait(100*time.Millisecond))
	for msg := range msgs2.Messages() {
		fmt.Printf("received %q\n", msg.Subject)
		msg.Ack()
	}

	// ## New JetStream API
	// Now let's look at the new JetStream API for creating and managing
	// streams and consumers.
	fmt.Println("\n# New API")

	// The new JetStream API is located in the `jetstream` package.
	// In order to create an entry point to the JetStream API, use the
	// `New` function.
	newJS, _ := jetstream.New(nc)

	// The new API uses `context.Context` for cancellation and timeouts when
	// managing streams and consumers.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Creating a stream is done using the `CreateStream` method.
	// It works similarly to the legacy `AddStream` method, except
	// instead of returning `StreamInfo`, it returns a `Stream` handle,
	// which can be used to manage the stream.
	// Instead of creating a new stream, let's look up the existing `EVENTS` stream.
	stream, _ := newJS.Stream(ctx, streamName)

	// The new API differs from the legacy API in that it does not
	// auto-create consumers. Instead, consumers must be created or retrieved
	// explicitly. This allows for more control over the consumer lifecycle,
	// while also getting rid of the hidden logic of the `Subscribe()` methods.
	// In order to create a consumer, use the `AddConsumer` method.
	// This method works similarly to the legacy `AddConsumer` method,
	// except it returns a `Consumer` handle, which can be used to manage
	// the consumer. Notice that since we are using pull consumers, we
	// do not need to provide a `DeliverSubject`.
	// In order to create a short-lived, ephemeral consumer, we will set the
	// `InactivityThreshold` to a low value and not provide a consumer name.
	cons, _ := stream.AddConsumer(ctx, jetstream.ConsumerConfig{
		InactiveThreshold: 10 * time.Millisecond,
	})
	fmt.Println("Created consumer", cons.CachedInfo().Name)

	// ### Continuous message retrieval with `Consume()`
	// In order to continuously receive messages, the `Consume` method
	// can be used. This method works similarly to the legacy `Subscribe`
	// method, in that it will asynchronously deliver messages to the
	// provided `jetstream.MsgHandler` function. However, it does not
	// create a consumer, instead it will use the consumer created
	// previously.
	fmt.Println("# Consume messages using Consume()")
	consumeContext, _ := cons.Consume(func(msg jetstream.Msg) {
		fmt.Printf("received %q\n", msg.Subject())
		msg.Ack()
	})
	time.Sleep(100 * time.Millisecond)

	// `Consume()` returns a `jetstream.ConsumerContext` which can be used
	// to stop consuming messages. In contrast to `Unsubscribe()` in the
	// legacy API, this will not delete the consumer.
	// Consumer will be automatically deleted by the server when the
	// `InactivityThreshold` is reached.
	consumeContext.Stop()

	// Now let's create a new, long-lived, named consumer.
	// In order to filter messages, we will provide a `FilterSubject`.
	// This is equivalent to providing a subject to `Subscribe` in the
	// legacy API.
	consumerName = "pull-1"
	cons, _ = stream.AddConsumer(ctx, jetstream.ConsumerConfig{
		Name:              consumerName,
		InactiveThreshold: 10 * time.Minute,
		FilterSubject:     "events.2",
	})
	fmt.Println("Created consumer", cons.CachedInfo().Name)

	// As an alternative to `Consume`, the `Messages()` method can be used
	// to retrieve messages one by one. Note that this method will
	// still pre-fetch messages, but instead of delivering them to a
	// handler function, it will return them upon calling `Next`.
	fmt.Println("# Consume messages using Messages()")
	it, _ := cons.Messages()
	msg1, _ := it.Next()
	fmt.Printf("received %q\n", msg1.Subject())

	// Similarly to `Consume`, `Messages` allows to stop consuming messages
	// without deleting the consumer.
	it.Stop()

	// ### Retrieving messages on demand with `Fetch()` and `Next()`

	// Similar to the legacy API, the new API also exposes a `Fetch()`
	// method for retrieving a specified number of messages on demand.
	// This method resembles the legacy `FetchBatch` method, in that
	// it will return a channel on which the messages will be delivered.
	fmt.Println("# Fetch messages")
	cons, _ = stream.AddConsumer(ctx, jetstream.ConsumerConfig{
		InactiveThreshold: 10 * time.Millisecond,
	})
	fetchResult, _ := cons.Fetch(2, jetstream.FetchMaxWait(100*time.Millisecond))
	for msg := range fetchResult.Messages() {
		fmt.Printf("received %q\n", msg.Subject())
		msg.Ack()
	}

	// Alternatively, the `Next` method can be used to retrieve a single
	// message. It works like `Fetch(1)`, returning a single message instead
	// of a channel.
	fmt.Println("# Get next message")
	msg1, _ = cons.Next()
	fmt.Printf("received %q\n", msg1.Subject())
	msg.Ack()

	// Streams and consumers can be deleted using the `DeleteStream` and
	// `DeleteConsumer` methods. Note that deleting a stream will also
	// delete all consumers on that stream.
	fmt.Println("# Delete consumer")
	stream.DeleteConsumer(ctx, cons.CachedInfo().Name)
	fmt.Println("# Delete stream")
	newJS.DeleteStream(ctx, streamName)
}
