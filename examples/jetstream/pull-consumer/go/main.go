package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	log.SetFlags(log.Default().Flags() | log.Lshortfile)
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

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	js.AddStream(&nats.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})

	// Publish a few messages for the example.
	js.Publish("events.1", nil)
	js.Publish("events.2", nil)
	js.Publish("events.3", nil)

	// The JetStreamContext provides a simple way to create an ephemeral
	// pull consumer. Simplify omit the subject and name and indicate the
	// stream to bind to. The subject _can_ be provided with `BindStream`
	// omitted as an alternative. In this case, the subject will be used
	// to lookup which stream the subject is bound to. Personally, I think
	// the use of bind is more explicit and readable.
	sub, _ := js.PullSubscribe("", "", nats.BindStream("EVENTS"))

	// We can _fetch_ messages in batches. The first argument being the
	// batch size which is the _maximum_ number of messages that should
	// be returned. For this first fetch, we ask for two and we will get
	// those since they are in the stream.
	msgs, err := sub.Fetch(2)
	if err != nil {
		log.Fatal(err) // <-- "nats: no responders available for request"??
	}
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
	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("timeout? %v", err == nats.ErrTimeout)

	// Unsubscribing this subscription will result in the ephemeral consumer
	// being deleted.
	sub.Drain()

	// Create a basic durable pull consumer by specifying the name and
	// the `Explicit` ack policy (which is required). The `AckWait` is only
	// added just to expedite one step in this example since it is normally
	// 30 seconds. Note, the `js.PullSubscribe` can be used to create a
	// durable consumer as well relying on subscription options, but this
	// shows a more explicit way. In addition, you can use the `UpdateConsumer`
	// method passing configuration to change on-demand.
	js.AddConsumer("EVENTS", &nats.ConsumerConfig{
		Durable:   "processor",
		AckPolicy: nats.AckExplicitPolicy,
	})

	// Unlike the `js.PullSubscribe` abstraction above, creating a consumer
	// still requires a subcription to be setup to actually process the
	// messages. This is done by using the `nats.Bind` option.
	sub1, err := js.PullSubscribe("", "", nats.Bind("EVENTS", "processor"))
	if err != nil {
		log.Fatal(err)
	}

	// All the same fetching works as above.
	msgs, _ = sub1.Fetch(1)
	fmt.Printf("received event on subject %q\n", msgs[0])
	msgs[0].Ack()

	// However, now we can unsubscribe and re-subscribe to pick up where
	// we left off.
	sub1.Unsubscribe()
	sub1, _ = js.PullSubscribe("", "", nats.Bind("EVENTS", "processor"))

	msgs, _ = sub1.Fetch(1)
	fmt.Printf("received event on subject %q\n", msgs[0])
	msgs[0].Ack()

	// We can also transparently add another subscription (typically in
	// a separate process) and fetch independently.
	sub2, _ := js.PullSubscribe("", "", nats.Bind("EVENTS", "processor"))

	msgs, _ = sub2.Fetch(1)
	fmt.Printf("received event on subject %q\n", msgs[0])
	msgs[0].Ack()

	// If we try to fetch from `sub1` again, notice we will timeout since
	// `sub2` processed the third message already.
	_, err = sub1.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("timeout? %v", err == nats.ErrTimeout)

	// Explicitly clean up. Note `nc.Drain()` above will do this automatically.
	sub1.Unsubscribe()
	sub2.Unsubscribe()
}
