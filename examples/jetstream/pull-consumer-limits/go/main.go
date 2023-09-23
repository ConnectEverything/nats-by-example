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

	// Access JetStream for managing streams and consumers
	// as well as for publishing and subscription convenience methods.
	js, _ := jetstream.New(nc)

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	streamName := "EVENTS"

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, _ := js.CreateStream(ctx, jetstream.StreamConfig{
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
	ackPolicy := jetstream.AckExplicitPolicy
	maxWaiting := 1

	// One quick note. This example show cases how consumer configuration
	// can be changed on-demand. This one exception is `MaxWaiting` which
	// cannot be updated on a consumer as of now. This must be set up front
	// when the consumer is created.
	cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:    consumerName,
		AckPolicy:  ackPolicy,
		AckWait:    ackWait,
		MaxWaiting: maxWaiting,
	})

	// ### Max in-flight messages
	// The first limit to explore is the max in-flight messages. This
	// will limit how many un-acked in-flight messages there are across
	// all subscriptions bound to this consumer.
	// We can update the consumer config on-the-fly with the
	// `MaxAckPending` setting.
	fmt.Println("--- max in-flight messages (n=1) ---")

	stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          consumerName,
		AckPolicy:     ackPolicy,
		AckWait:       ackWait,
		MaxWaiting:    maxWaiting,
		MaxAckPending: 1,
	})

	// Let's publish a couple events for this section.
	js.Publish(ctx, "events.1", nil)
	js.Publish(ctx, "events.2", nil)

	// We can request a larger batch size, but we will only get one
	// back since only one can be un-acked at any given time. This
	// essentially forces serial processing messages for a pull consumer.
	msgs, _ := cons.FetchNoWait(3)

	var received []jetstream.Msg
	for msg := range msgs.Messages() {
		received = append(received, msg)
	}
	fmt.Printf("requested 3, got %d\n", len(received))

	// This limit becomes more apparent with the second fetch which would
	// timeout without any messages since we haven't acked the previous one yet.
	msgs, _ = cons.Fetch(1, jetstream.FetchMaxWait(time.Second))
	var received2 []jetstream.Msg
	for msg := range msgs.Messages() {
		received2 = append(received2, msg)
	}
	fmt.Printf("requested 1, got %d\n", len(received2))

	// Let's ack it and then try another fetch.
	received[0].Ack()

	// It works this time!
	msgs, _ = cons.Fetch(1)
	for msg := range msgs.Messages() {
		received2 = append(received2, msg)
		msg.Ack()
	}
	fmt.Printf("requested 1, got %d\n", len(received2))

	// ### Max fetch batch size
	// This one limits the max batch size any one fetch can receive. This
	// can be used to keep the fetches to a reasonable size.
	fmt.Println("\n--- max fetch batch size (n=2) ---")

	cons, _ = stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:            consumerName,
		AckPolicy:       ackPolicy,
		AckWait:         ackWait,
		MaxWaiting:      maxWaiting,
		MaxRequestBatch: 2,
	})

	// Publish a couple events for this section...
	js.Publish(ctx, "events.1", []byte("hello"))
	js.Publish(ctx, "events.2", []byte("world"))

	// If a batch size is larger than the limit, it is considered an error.
	// Because Fetch is non-blocking, we need to wait for the operation to
	// complete before checking the error.
	msgs, _ = cons.Fetch(10)
	for range msgs.Messages() {
	}
	fmt.Printf("%s\n", msgs.Error())

	// Using the max batch size (or less) will, of course, work.
	msgs, _ = cons.Fetch(2)
	var i int
	for msg := range msgs.Messages() {
		fmt.Printf("received %q\n", msg.Data())
		msg.Ack()
		i++
	}
	fmt.Printf("requested 2, got %d\n", i)

	// ### Max waiting requests
	// The next limit defines the maximum number of fetch requests
	// that are all waiting in parallel to receive messages. This
	// prevents building up too many requests that the server will
	// have to distribute to for a given consumer.
	fmt.Println("\n--- max waiting requests (n=1) ---")

	// Since `MaxWaiting` was already set to 1 when the consumer
	// was created, this is a no-op.
	stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:       consumerName,
		AckPolicy:  ackPolicy,
		AckWait:    ackWait,
		MaxWaiting: maxWaiting,
	})

	msgs1, _ := cons.Fetch(1, jetstream.FetchMaxWait(time.Second))
	msgs2, _ := cons.Fetch(1, jetstream.FetchMaxWait(time.Second))
	msgs3, _ := cons.Fetch(1, jetstream.FetchMaxWait(time.Second))

	time.Sleep(1 * time.Second)

	fmt.Printf("fetch 1: %v\n", msgs1.Error())
	fmt.Printf("fetch 2: %v\n", msgs2.Error())
	fmt.Printf("fetch 3: %v\n", msgs3.Error())

	// ### Max fetch timeout
	// Normally each fetch call can specify it's own max wait timeout, i.e.
	// how long the client wants to wait to receive at least one message.
	// It may be desirable to limit defined on the consumer to prevent
	// requests waiting too long for messages.
	fmt.Println("\n--- max fetch timeout (d=1s) ---")

	stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:              consumerName,
		AckPolicy:         ackPolicy,
		AckWait:           ackWait,
		MaxWaiting:        maxWaiting,
		MaxRequestExpires: time.Second,
	})

	// Using a max wait equal or less than `MaxRequestExpires` not return an
	// error and return expected number of messages (zero in that case, since
	// there are no more).
	t0 := time.Now()
	msgs, _ = cons.Fetch(1, jetstream.FetchMaxWait(time.Second))
	for range msgs.Messages() {
	}
	fmt.Printf("error? %v in %s\n", msgs.Error() != nil, time.Since(t0))

	// However, trying to use a longer timeout will return immediately with
	// an error.
	t0 = time.Now()
	msgs, _ = cons.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	for range msgs.Messages() {
	}
	fmt.Printf("%s\n", msgs.Error())

	// ### Max total bytes per fetch
	//
	fmt.Println("\n--- max total bytes per fetch (n=4) ---")

	stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:               consumerName,
		AckPolicy:          ackPolicy,
		AckWait:            ackWait,
		MaxWaiting:         maxWaiting,
		MaxRequestMaxBytes: 3,
	})

	js.Publish(ctx, "events.3", []byte("hola"))
	js.Publish(ctx, "events.4", []byte("again"))

	msgs, _ = cons.FetchBytes(4)
	for range msgs.Messages() {
	}
	fmt.Printf("%s\n", msgs.Error())
}
