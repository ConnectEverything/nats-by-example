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
	defer nc.Drain()

	// Access the JetStreamContext for managing streams and consumers
	// as well as for publishing and subscription convenience methods.
	js, _ := nc.JetStream()

	streamName := "EVENTS"

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"events.>"},
	})

	// Publish a few messages for the example.
	js.Publish("events.1", []byte("changed"))
	js.Publish("events.2", []byte("fixed"))
	js.Publish("events.3", []byte("updated"))

	// The JetStreamContext provides a simple way to create an ephemeral
	// push consumer, simply provide a subject that overlaps with the
	// bound subjects on the stream and this helper method will do the
	// stream look-up automatically and create the consumer.
	sub, _ := js.SubscribeSync("events.>")

	// An ephemeral consumer has a name generated on the server-side.
	// Since there is only one consumer so far, let's just get the first
	// one.
	ephemeralName := <-js.ConsumerNames(streamName)
	fmt.Printf("ephemeral name is %q\n", ephemeralName)

	// Since this is a push consumer, messages will be sent by the server
	// and pre-buffered by this subscription. We can observe this by using
	// the `Pending()` method. The bytes are the sum of the message data
	// in bytes.
	queuedMsgs, queuedBytes, _ := sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// The number of messages that will be queued is defined by the
	// `MaxAckPending` option set on a consumer. The default is 65,536.
	// Let's observe this by publishing a few more events and then check
	// the pending status again.
	js.Publish("events.4", []byte("handled"))
	js.Publish("events.5", []byte("received"))
	js.Publish("events.6", []byte("clicked"))

	// Although messages pushed from the server is asynchronous with
	// respect to the subscriptions, for this basic example, we will
	// observe they have been received already.
	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// To receive a message, call `NextMsg` with a timeout. The timeout
	// applies when the consumer has fully caught up to the available
	// messages in the stream. If no messages become available, this call
	// will only block until the timeout.
	msg, _ := sub.NextMsg(time.Second)
	fmt.Printf("received %q\n", msg.Subject)

	// By default, the underlying consumer requires explicit acknowlegements,
	// otherwise messges will get redelivered.
	msg.Ack()

	// Let's receive and ack another.
	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received %q\n", msg.Subject)
	msg.Ack()

	// Checking out our pending information, we see there are only four
	// remaining.
	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// Unsubscribing this subscription will result in the ephemeral consumer
	// being deleted. Note, even if this is omitted and the process ends
	// or is interrupted, the server will eventually clean-up the ephemeral
	// when it determines the subscription is not long active.
	sub.Unsubscribe()

	// We can use the same `SubscribeSync` method to create a durable
	// consumer as well by passing `nats.Durable()`. This will implicitly
	// create the durable if it doesn't exist, otherwise it will bind to
	// an existing one if created.
	sub, _ = js.SubscribeSync("events.>", nats.Durable("event-handler"))

	// Let's check out pending messages again. We should have all six
	// queued up.
	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// Let's receive one and ack it.
	msg, _ = sub.NextMsg(time.Second)
	msg.Ack()

	// Can we bind another subscription to this consumer? Nope 🙂.
	// For standard push consumer, only one subscription can be bound at
	// any given time. If there is a need for multiple subscriptions to
	// distribute message processing see `QueueSubscribe*` (coming in a
	// future example).
	_, err := js.SubscribeSync("events.>", nats.Durable("event-handler"))
	fmt.Println(err)

	// Another nuance of implicitly creating a durable using this helper
	// method is that if `Unsubscribe` or `Drain` is called, the consumer
	// will actually be deleted. So these helpers should only be used
	// if neither of those methods are called.
	// A more explicit and (in my opinion) safer way to create durables
	// is using `js.AddConsumer`.
	// For push consumers, we must provide a `DeliverSubject` which is the
	// subject messages will be published to for a subscription to receive
	// them. We will also use a different durable name...
	consumerName := "event-handler-2"
	js.AddConsumer(streamName, &nats.ConsumerConfig{
		Durable:        consumerName,
		DeliverSubject: "handler-2",
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        500 * time.Millisecond,
	})

	// Now there are few ways we can setup a subscription. The two most
	// straightforward ways are using `nats.Bind` which handles looking
	// up the consumer and creating a core NATS subscription with the
	// `DeliverSubject`
	sub, _ = js.SubscribeSync("", nats.Bind(streamName, consumerName))

	// Receive, ack, and show pending.
	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received %q\n", msg.Subject)
	msg.Ack()

	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// Since we created the durable explicitly, we can freely unsubscribe
	// and re-subscribe without the consumer being deleted.
	sub.Unsubscribe()

	// The second way to subscribe is directly to the `DeliverSubject` with
	// a core NATS subscription.
	sub, _ = js.SubscribeSync("", nats.Bind(streamName, consumerName))

	// Upon resubscribe we would expect our queue to be at five, right??
	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// 😨 Well, not yet. This is an area where people get tripped up with
	// push consumers. As note above, messages will be pushed to the
	// subscription *proactively*. These are expected to be handled, but
	// if they are not (in the case of us unsubscribing early only after
	// handling one message), those messages are not effectively in limbo
	// until the `MaxAck` timeout is reached. By default this is 30 seconds
	// for consumers requiring an explicit ack.
	// So we are not waiting for 30 seconds, I had explicity set the
	// `MaxAck` option in the consumer config above to shorter time so
	// we can simulate and observe the redelivery.
	time.Sleep(time.Second)

	// Now that the ack timeout has been reached, the server will redeliver
	// to this new active subscription. We can see this by checking the
	// pending messages again.
	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)

	// The next message we receive will be the second event since the first
	// was consumed by the previous subscription.
	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received %q\n", msg.Subject)
	msg.Ack()

	queuedMsgs, queuedBytes, _ = sub.Pending()
	fmt.Printf("%d messages queued (%d bytes)\n", queuedMsgs, queuedBytes)
}
