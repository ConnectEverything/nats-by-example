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
	js.Publish("events.1", nil)
	js.Publish("events.2", nil)
	js.Publish("events.3", nil)

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
	// the `Pending()` method.
	queuedMsgs, _, _ := sub.Pending()
	fmt.Printf("%d messages queued\n", queuedMsgs)

	// The number of messages that will be queued is defined by the
	// `MaxAckPending` option set on a consumer. The default is 65,536.
	// Let's observe this by publishing a few more events and then check
	// the pending status again.
	js.Publish("events.4", nil)
	js.Publish("events.5", nil)
	js.Publish("events.6", nil)

	// Although messages pushed from the server is asynchronous with
	// respect to the subscriptions, for this basic example, we can
	// observe they have been received already.
	queuedMsgs, _, _ = sub.Pending()
	fmt.Printf("%d messages queued\n", queuedMsgs)

	// To *receive* a message, call `NextMsg` with a timeout. The timeout
	// applies when pending count is zero and the consumer has fully caught
	// up to the available messages in the stream. If no messages become
	// available, this call will only block until the timeout.
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
	queuedMsgs, _, _ = sub.Pending()
	fmt.Printf("%d messages queued\n", queuedMsgs)

	// Unsubscribing this subscription will result in the ephemeral consumer
	// being deleted. Note, even if this is omitted and the process ends
	// or is interrupted, the server will eventually clean-up the ephemeral
	// when it determines the subscription is no longer active.
	sub.Unsubscribe()

	// We can use the same `SubscribeSync` method to create a durable
	// consumer as well by passing `nats.Durable()`. This will implicitly
	// create the durable if it does not exist, otherwise it will bind to
	// an existing one if it exist.
	sub, _ = js.SubscribeSync("events.>", nats.Durable("handler-1"))

	// Let's check out pending messages again. We should have all six
	// queued up.
	queuedMsgs, _, _ = sub.Pending()
	fmt.Printf("%d messages queued\n", queuedMsgs)

	// Let's receive one and ack it.
	msg, _ = sub.NextMsg(time.Second)
	msg.Ack()

	// One nuance of implicitly creating a durable using this helper
	// method is that if `Unsubscribe` or `Drain` is called, the consumer
	// will actually be deleted. So these helpers should only be used
	// if neither of those methods are called.
	sub.Unsubscribe()

	// If we try to get the consumer info, we will see it no longer exists.
	_, err := js.ConsumerInfo("EVENTS", "handler-1")
	fmt.Println(err)

	// A more explicit and safer way to create durables is using `js.AddConsumer`.
	// For push consumers, we must provide a `DeliverSubject` which is the
	// subject messages will be published to (pushed) for a subscription to
	// receive them. Another best practice is to use an AckExplicit or AckAll
	// policy depending on your use case. This will provide more explicit control
	// over acking.
	consumerName := "handler-2"
	js.AddConsumer(streamName, &nats.ConsumerConfig{
		Durable:        consumerName,
		DeliverSubject: "handler-2",
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        time.Second,
	})

	// Now that the consumer is created, we need to bind a client subscription
	// to it which will receive and process the messages. This can be done
	// using the `nats.Bind` subscription option which requires the consumer
	// to have been pre-created. The subject can be omitted since that was
	// already defined on the consumer. Subscriptions to consumers cannot
	// independently define their own subject to filter on.
	sub, _ = js.SubscribeSync("", nats.Bind(streamName, consumerName))

	// The next step is to receive a message which can be done using
	// the `NextMsg` method. The passed duration is the amount of time
	// to wait before until a message is received. This is received because
	// `SubscribeSync` is the _synchronous_ form of a push consumer
	// subscription. There is also the `Subscribe` variant which takes
	// a `nats.MsgHandler` function to receive and process messages
	// asynchronously, but that will be described in a different example.
	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received %q\n", msg.Subject)

	// Let's ack the message and check out the pending count.
	msg.Ack()
	queuedMsgs, _, _ = sub.Pending()
	fmt.Printf("%d messages queued\n", queuedMsgs)

	// If we unsubscribe/drain, what happens to these pending messages?
	// From the client's perspective they are effectively dropped. This behavior
	// would be true if the client crashed for some reason.
	// From the server's perspective it is going to wait until `AckWait`
	// before attempting to re-deliver them. However, it will only re-deliver
	// if there is an active subscription.
	sub.Drain()

	// If we check out the consumer info, we can pull out a few interesting
	// bits of information. The first one is that the consumer tracks the
	// sequence of the last message in the _stream_ that a delivery was
	// attempted for. The second is that it maintains its own sequence to
	// track delivery attempts. These should not be treated as correlated
	// since the consumer sequence for a given message will increment on
	// each delivery attempt.
	// The "num ack pending" indicates how many messages have been delivered
	// and awaiting an acknowledgement. Since we ack'ed one already, there
	// are five remaining.
	// The final one to note here are the number of redeliveries. Since
	// these messages have been only delivered once (so far) for this consumer
	// this value is zero.
	info, _ := sub.ConsumerInfo()
	fmt.Printf("max stream sequence delivered: %d\n", info.Delivered.Stream)
	fmt.Printf("max consumer sequence delivered: %d\n", info.Delivered.Consumer)
	fmt.Printf("num ack pending: %d\n", info.NumAckPending)
	fmt.Printf("num redelivered: %d\n", info.NumRedelivered)

	// If we create a new subscription and attempt to get a message
	// before the AckWait, we will get a timeout.
	sub, _ = js.SubscribeSync("", nats.Bind(streamName, consumerName))
	_, err = sub.NextMsg(100 * time.Millisecond)
	fmt.Printf("received timeout? %v\n", err == nats.ErrTimeout)

	// Let's try again and wait a bit longer beyond the AckWait.
	msg, _ = sub.NextMsg(time.Second)
	md, _ := msg.Metadata()
	fmt.Printf("received %q (delivery #%d)\n", msg.Subject, md.NumDelivered)
	msg.Ack()

	// We can see how the numbers changed by viewing the consumer info
	// again.
	info, _ = sub.ConsumerInfo()
	fmt.Printf("max stream sequence delivered: %d\n", info.Delivered.Stream)
	fmt.Printf("max consumer sequence delivered: %d\n", info.Delivered.Consumer)
	fmt.Printf("num ack pending: %d\n", info.NumAckPending)
	fmt.Printf("num redelivered: %d\n", info.NumRedelivered)
}
