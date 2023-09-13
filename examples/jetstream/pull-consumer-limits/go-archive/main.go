package main

import (
	"fmt"
	"os"
	"sync"
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

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	streamName := "EVENTS"

	js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"events.>"},
	})

	// Define a basic pull consumer without any limits and a short ack wait
	// time for the purpose of this example. These default options will be
	// reused when we update the consumer to show-case various limits.
	// If you haven't seen the first [pull consumer][1] example yet, check
	// that out first!
	// [1]: /examples/jetstream/pull-consumer/go/
	consumerName := "processor"
	ackWait := 10 * time.Second
	ackPolicy := nats.AckExplicitPolicy
	maxWaiting := 1

	// One quick note. This example show cases how consumer configuration
	// can be changed on-demand. This one exception is `MaxWaiting` which
	// cannot be updated on a consumer as of now. This must be set up front
	// when the consumer is created.
	js.AddConsumer(streamName, &nats.ConsumerConfig{
		Durable:    consumerName,
		AckPolicy:  ackPolicy,
		AckWait:    ackWait,
		MaxWaiting: maxWaiting,
	})

	// Bind a subscription to the consumer.
	sub, _ := js.PullSubscribe("", consumerName, nats.Bind(streamName, consumerName))

	// ### Max in-flight messages
	// The first limit to explore is the max in-flight messages. This
	// will limit how many un-acked in-flight messages there are across
	// all subscriptions bound to this consumer.
	// We can update the consumer config on-the-fly with the
	// `MaxAckPending` setting.
	fmt.Println("--- max in-flight messages (n=1) ---")

	js.UpdateConsumer(streamName, &nats.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     ackPolicy,
		AckWait:       ackWait,
		MaxWaiting:    maxWaiting,
		MaxAckPending: 1,
	})

	// Let's publish a couple events for this section.
	js.Publish("events.1", nil)
	js.Publish("events.2", nil)

	// We can request a larger batch size, but we will only get one
	// back since only one can be un-acked at any given time. This
	// essentially forces serial processing messages for a pull consumer.
	msgs, _ := sub.Fetch(3)
	fmt.Printf("requested 3, got %d\n", len(msgs))

	// This limit becomes more apparent with the second fetch which would
	// timeout since we haven't acked the previous one yet.
	_, err := sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("%s\n", err)

	// Let's ack it and then try another fetch.
	msgs[0].Ack()

	// It works this time!
	msgs, _ = sub.Fetch(1)
	fmt.Printf("requested 1, got %d\n", len(msgs))
	msgs[0].Ack()

	// ### Max fetch batch size
	// This one limits the max batch size any one fetch can receive. This
	// can be used to keep the fetches to a reasonable size.
	fmt.Println("\n--- max fetch batch size (n=2) ---")

	js.UpdateConsumer(streamName, &nats.ConsumerConfig{
		Durable:         consumerName,
		AckPolicy:       ackPolicy,
		AckWait:         ackWait,
		MaxWaiting:      maxWaiting,
		MaxRequestBatch: 2,
	})

	// Publish a couple events for this section...
	js.Publish("events.1", []byte("hello"))
	js.Publish("events.2", []byte("world"))

	// If a batch size is larger than the limit, it is considered an error.
	_, err = sub.Fetch(10)
	fmt.Printf("%s\n", err)

	// Using the max batch size (or less) will, of course, work.
	msgs, _ = sub.Fetch(2)
	fmt.Printf("requested 2, got %d\n", len(msgs))

	msgs[0].Ack()
	msgs[1].Ack()

	// ### Max waiting requests
	// The next limit defines the maximum number of fetch requests
	// that are all waiting in parallel to receive messages. This
	// prevents building up too many requests that the server will
	// have to distribute to for a given consumer.
	fmt.Println("\n--- max waiting requests (n=1) ---")

	// Since `MaxWaiting` was already set to 1 when the consumer
	// was created, this is a no-op.
	js.UpdateConsumer(streamName, &nats.ConsumerConfig{
		Durable:    consumerName,
		AckPolicy:  ackPolicy,
		AckWait:    ackWait,
		MaxWaiting: maxWaiting,
	})

	fmt.Printf("is valid? %v\n", sub.IsValid())

	// Since `Fetch` is blocking, we will put these in a few goroutines
	// to simulate the behavior. There are no more messages in the stream
	// so we will expect two max waiting errors and one timeout.
	wg := &sync.WaitGroup{}
	wg.Add(3)

	go func() {
		_, err := sub.Fetch(1, nats.MaxWait(time.Second))
		fmt.Printf("fetch 1: %s\n", err)
		wg.Done()
	}()

	go func() {
		_, err := sub.Fetch(1, nats.MaxWait(time.Second))
		fmt.Printf("fetch 2: %s\n", err)
		wg.Done()
	}()

	go func() {
		_, err := sub.Fetch(1, nats.MaxWait(time.Second))
		fmt.Printf("fetch 3: %s\n", err)
		wg.Done()
	}()

	wg.Wait()

	// ### Max fetch timeout
	// Normally each fetch call can specify it's own max wait timeout, i.e.
	// how long the client wants to wait to receive at least one message.
	// It may be desirable to limit defined on the consumer to prevent
	// requests waiting too long for messages.
	fmt.Println("\n--- max fetch timeout (d=1s) ---")

	js.UpdateConsumer(streamName, &nats.ConsumerConfig{
		Durable:           consumerName,
		AckPolicy:         ackPolicy,
		AckWait:           ackWait,
		MaxWaiting:        maxWaiting,
		MaxRequestExpires: time.Second,
	})

	// Using a max wait equal or less than `MaxRequestExpires` will result
	// in the expected timeout since there are no messages currently.
	t0 := time.Now()
	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("timeout occured? %v in %s\n", err == nats.ErrTimeout, time.Since(t0))

	// However, trying to use a longer timeout will return immediately with
	// an error.
	t0 = time.Now()
	_, err = sub.Fetch(1, nats.MaxWait(5*time.Second))
	fmt.Printf("%s in %s\n", err, time.Since(t0))

	// ### Max total bytes per fetch
	//
	fmt.Println("\n--- max total bytes per fetch (n=4) ---")

	js.UpdateConsumer(streamName, &nats.ConsumerConfig{
		Durable:            consumerName,
		AckPolicy:          ackPolicy,
		AckWait:            ackWait,
		MaxWaiting:         maxWaiting,
		MaxRequestMaxBytes: 3,
	})

	js.Publish("events.3", []byte("hola"))
	js.Publish("events.4", []byte("again"))

	_, err = sub.Fetch(2, nats.PullMaxBytes(4))
	fmt.Printf("%s\n", err)
}
