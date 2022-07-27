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

	// Access `JetStreamContext` which provides methods to create
	// streams and consumers as well as convenience methods for publishing
	// to streams and implicitly creating consumers through `*Subscribe*`
	// methods (which will be discussed in examples focused on consumers).
	js, _ := nc.JetStream()

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	js.AddStream(&nats.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})

	// Define the base config for the pull consumer without any limits
	// and a short ack wait time for the purpose of this example.
	cfg := &nats.ConsumerConfig{
		Durable:   "processor",
		AckPolicy: nats.AckExplicitPolicy,
		AckWait:   5 * time.Second,
	}

	js.AddConsumer("EVENTS", cfg)

	// Bind a subscription to the consumer.
	sub, _ := js.PullSubscribe("", "processor", nats.Bind("EVENTS", "processor"))

	// Per call, we can vary the batch size and the max wait for a response.
	// By default, the max wait is 30 seconds. We know there are no messages
	// in the stream, so we shorten this max wait to show the timeout error.
	fmt.Println("expecting a timeout...")
	_, err := sub.Fetch(5, nats.MaxWait(time.Second))
	fmt.Println(err)

	// Let's publish some events using the helper function below and then fetch
	// again.
	publishEvents(js, 20)

	fmt.Println("try fetching again...")
	batch1, _ := sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("received %d messages\n", len(batch1))

	// The messages haven't been ack'ed yet, what happens if I fetch again
	// and with a larger batch size? We know there are only 20 events in
	// the stream...
	// If at least one message can be fetched, it will returned. So the fetch
	// size is the _maximum_ number of messages that should be fetched in one
	// request.
	fmt.Println("fetching again...")
	batch2, _ := sub.Fetch(20, nats.MaxWait(time.Second))
	fmt.Printf("requested 20, got %d\n", len(batch2))

	// Since we set the *ack wait* on the consumer config, let's
	// sleep and then try to fetch again..
	fmt.Println("sleeping for ack wait..")
	time.Sleep(cfg.AckWait)

	// Since the ack wait time was exceeded, the messages are eligible
	// for redelivery and that is what is happening here.
	fmt.Println("fetching after sleep...")
	batch1, _ = sub.Fetch(20, nats.MaxWait(time.Second))
	fmt.Printf("received all %d messages\n", len(batch1))

	// We can confirm that by looking at the message metadata for the
	// *num delivered* field which will be 2 since this is the second time
	// it was delivered.
	md, _ := batch1[0].Metadata()
	fmt.Printf("num delivered == 2? %v\n", md.NumDelivered == 2)

	// Let's ack them this time and publish some fresh events.
	ackMsgs(batch1)
	publishEvents(js, 20)

	// ### Max in-flight messages
	// The first limit to explore is the max in-flight messages. This
	// will limit how many un-acked in-flight messages there are. We
	// can update the consumer config after setting this value.
	// (note only certain options can be changed).
	cfg.MaxAckPending = 5
	js.UpdateConsumer("EVENTS", cfg)

	fmt.Println("--- max in-flight messages ---")

	// Now if try to fetch more than five messages, we will only be given
	// five since none are currently in-flight.
	batch1, _ = sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("requested 10, got %d\n", len(batch1))

	// This limit becomes more apparent with the second fetch which would
	// timeout since we haven't acked the previous five yet.
	_, err = sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("timeout error? %v\n", err == nats.ErrTimeout)

	// Once we ack the first batch, then we can fetch some more.
	ackMsgs(batch1)

	// Let's fetch the remaining 10 and ack them..
	batch1, _ = sub.Fetch(5, nats.MaxWait(time.Second))
	ackMsgs(batch1)
	batch1, _ = sub.Fetch(5, nats.MaxWait(time.Second))
	ackMsgs(batch1)

	// Publish some fresh events for the next option.
	publishEvents(js, 20)

	// ### Max fetch batch size
	// This one simply limits the max batch size any one fetch can receive.
	// We will also reset the max ack pending so we don't confuse which limit
	// is taking effect.
	cfg.MaxRequestBatch = 5
	cfg.MaxAckPending = 0
	js.UpdateConsumer("EVENTS", cfg)

	fmt.Println("--- max fetch batch size ---")

	// Note that although multiple requests can be made (without acking),
	// each one will return at most the max request batch size.
	batch1, _ = sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("requested 10, got %d\n", len(batch1))

	batch2, err = sub.Fetch(6, nats.MaxWait(time.Second))
	fmt.Printf("requested 6, got %d -> %s\n", len(batch2), err)

	batch3, err := sub.Fetch(6, nats.MaxWait(time.Second))
	fmt.Printf("requested 6, got %d -> %s\n", len(batch3), err)

	batch4, err := sub.Fetch(6, nats.MaxWait(time.Second))
	fmt.Printf("requested 6, got %d -> %s\n", len(batch4), err)

	ackMsgs(batch1)
	batch1, err = sub.Fetch(10)
	if err != nil {
		fmt.Println(err)
	}
	ackMsgs(batch1)

	// Publish some fresh events for the next option.
	publishEvents(js, 20)

	// ### Max in-flight requests
	// The second limit we will add constrains the number of in-flight
	// pull requests with unack'ed messages across subscribers. We will
	// also revert the max ack pending to focus on this new limit.
	cfg.MaxWaiting = 2
	cfg.MaxRequestBatch = 0
	js.UpdateConsumer("EVENTS", cfg)

	fmt.Println("--- max in-flight requests ---")

	// For the current subscription we have, we can make more than *max
	// waiting* fetch requests since the configuration applies across
	// subscriptions.
	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("sub1: fetch 1 ok? %v\n", err == nil)

	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("sub1: fetch 2 ok? %v\n", err == nil)

	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("sub1: fetch 3 ok? %v\n", err == nil)

	// Adding a second subscription, the fetch also succeeds since this is
	// only the second one.
	sub2, _ := js.PullSubscribe("", "processor", nats.Bind("EVENTS", "processor"))
	_, err = sub2.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("sub2: fetch 1 ok? %v\n", err == nil)
	sub2.Drain()

	// However, adding the third subscription and fetching will fail.
	sub3, _ := js.PullSubscribe("", "processor", nats.Bind("EVENTS", "processor"))
	_, err = sub3.Fetch(1, nats.MaxWait(time.Second))
	fmt.Printf("sub3: fetch 1 ok? %v -> %s\n", err == nil, err)
	sub3.Drain()

	// ### Max fetch timeout
	// Shown above, we are explicitly setting the MaxWait option on per
	// fetch request, however this can be set and capped as part of the
	// consumer config.
	cfg.MaxRequestExpires = time.Second
	cfg.MaxWaiting = 0
	js.UpdateConsumer("events", cfg)

	// Clean up and ack all messages to demonstrate the option.
	batch1, _ = sub.Fetch(100)
	ackMsgs(batch1)

	// All messages have been acked in the stream, so this request should
	// timeout. We will set a longer MaxWait time to demonstrate.
	t0 := time.Now()
	_, err = sub.Fetch(10, nats.MaxWait(3*time.Second))
	fmt.Printf("timeout occured? %v in %s\n", err == nats.ErrTimeout, time.Since(t0))

	// ### Max total bytes per fetch
	// This is not yet implememented...
}

// Helper function to publish some events to the stream.
func publishEvents(js nats.JetStreamContext, count int) {
	for i := 0; i < count; i++ {
		subject := fmt.Sprintf("events.%d", i%5)
		js.PublishAsync(subject, nil)
	}

	<-js.PublishAsyncComplete()
	fmt.Printf("published %d messages\n", count)
}

func ackMsgs(msgs []*nats.Msg) {
	for _, m := range msgs {
		m.Ack()
	}
	fmt.Printf("acked %d messages\n", len(msgs))
}
