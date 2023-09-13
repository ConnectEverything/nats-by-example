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
	// pull consumer. Simply durable name (second parameter) and specify
	// either the subject or the explicit stream to bind to using `BindStream`.
	// If the subject is provided, it will be used to look up the stream
	// the subject is bound to.
	sub, _ := js.PullSubscribe("", "", nats.BindStream(streamName))

	// An ephemeral consumer has a name generated on the server-side.
	// Since there is only one consumer so far, let's just get the first
	// one.
	ephemeralName := <-js.ConsumerNames(streamName)
	fmt.Printf("ephemeral name is %q\n", ephemeralName)

	// We can _fetch_ messages in batches. The first argument being the
	// batch size which is the _maximum_ number of messages that should
	// be returned. For this first fetch, we ask for two and we will get
	// those since they are in the stream.
	msgs, _ := sub.Fetch(2)
	fmt.Printf("got %d messages\n", len(msgs))

	// Let's also ack them so they are not redelivered.
	msgs[0].Ack()
	msgs[1].Ack()

	// Even though we are requesting a large batch (we know there is only
	// one message left in the stream), we will get that one message right
	// away.
	msgs, _ = sub.Fetch(100)
	fmt.Printf("got %d messages\n", len(msgs))

	msgs[0].Ack()

	// Finally, if we are at the end of the stream and we call fetch,
	// the call will be blocked until the "max wait" time which is 5
	// seconds by default, but this can be set explicitly as an option.
	_, err := sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("timeout? %v\n", err == nats.ErrTimeout)

	// Unsubscribing this subscription will result in the ephemeral consumer
	// being deleted.
	sub.Unsubscribe()

	// Create a basic durable pull consumer by specifying the name and
	// the `Explicit` ack policy (which is required). The `AckWait` is only
	// added just to expedite one step in this example since it is normally
	// 30 seconds. Note, the `js.PullSubscribe` can be used to create a
	// durable consumer as well relying on subscription options, but this
	// shows a more explicit way. In addition, you can use the `UpdateConsumer`
	// method passing configuration to change on-demand.
	consumerName := "processor"
	js.AddConsumer(streamName, &nats.ConsumerConfig{
		Durable:   consumerName,
		AckPolicy: nats.AckExplicitPolicy,
	})

	// Unlike the `js.PullSubscribe` abstraction above, creating a consumer
	// still requires a subcription to be setup to actually process the
	// messages. This is done by using the `nats.Bind` option.
	sub1, _ := js.PullSubscribe("", consumerName, nats.BindStream(streamName))

	// All the same fetching works as above.
	msgs, _ = sub1.Fetch(1)
	fmt.Printf("received %q from sub1\n", msgs[0].Subject)
	msgs[0].Ack()

	// However, now we can unsubscribe and re-subscribe to pick up where
	// we left off.
	sub1.Unsubscribe()
	sub1, _ = js.PullSubscribe("", consumerName, nats.BindStream(streamName))

	msgs, _ = sub1.Fetch(1)
	fmt.Printf("received %q from sub1 (after reconnect)\n", msgs[0].Subject)
	msgs[0].Ack()

	// We can also transparently add another subscription (typically in
	// a separate process) and fetch independently.
	sub2, _ := js.PullSubscribe("", consumerName, nats.BindStream(streamName))

	msgs, _ = sub2.Fetch(1)
	fmt.Printf("received %q from sub2\n", msgs[0].Subject)
	msgs[0].Ack()

	// If we try to fetch from `sub1` again, notice we will timeout since
	// `sub2` processed the third message already.
	_, err = sub1.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("timeout on sub1? %v\n", err == nats.ErrTimeout)

	// Explicitly clean up. Note `nc.Drain()` above will do this automatically.
	sub1.Unsubscribe()
	sub2.Unsubscribe()
}
