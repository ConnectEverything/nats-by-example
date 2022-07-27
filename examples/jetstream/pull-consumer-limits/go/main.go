package main

import (
	"fmt"
	"log"
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

	// Drain is a safe way to to ensure all buffered messages that were published
	// are sent and all buffered messages received on a subscription are processed
	// being closing the connection.
	defer nc.Drain()

	// Access `JetStreamContext` which provides methods to create
	// streams and consumers as well as convenience methods for publishing
	// to streams and implicitly creating consumers through `*Subscribe*`
	// methods (which will be discussed in examples focused on consumers).
	js, _ := nc.JetStream()

	streamName := "events"
	consumerName := "processor"
	updateConfig := false

	sub := resetStreamConsumerSub(js, nil, streamName, &nats.ConsumerConfig{
		Durable:   consumerName,
		AckPolicy: nats.AckExplicitPolicy,
		AckWait:   5 * time.Second,
	}, updateConfig)

	// Per call, we can vary the batch size and the max wait for a response.
	// By default, the max wait is 30 seconds. We know there are no messages
	// in the stream, so we shorten this max wait to show the timeout error.
	fmt.Println("expecting a timeout...")
	_, err := sub.Fetch(5, nats.MaxWait(time.Second))
	fmt.Printf("timeout received? %v\n", err == nats.ErrTimeout)

	// Let's publish some events using the helper function below and then fetch
	// again.
	publishEvents(js, streamName, 20)

	fmt.Println("try fetching again...")
	batch, _ := sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("received %d messages\n", len(batch))
	checkRedelivered(batch)

	// The messages haven't been ack'ed yet, what happens if I fetch again
	// and with a larger batch size? We know there are only 20 events in
	// the stream...
	// If at least one message can be fetched, it will returned. So the fetch
	// size is the _maximum_ number of messages that should be fetched in one
	// request.
	fmt.Println("fetching again...")
	batch, _ = sub.Fetch(20, nats.MaxWait(time.Second))
	fmt.Printf("requested 20, got %d\n", len(batch))
	checkRedelivered(batch)

	// Since we set the *ack wait* on the consumer config, let's
	// sleep and then try to fetch again..
	fmt.Println("sleeping for ack wait..")
	time.Sleep(5 * time.Second)

	// Since the ack wait time was exceeded, the messages are eligible
	// for redelivery and that is what is happening here.
	fmt.Println("fetching after sleep...")
	batch, _ = sub.Fetch(20, nats.MaxWait(time.Second))
	fmt.Printf("received all %d messages\n", len(batch))
	checkRedelivered(batch)
	ackMsgs(batch)

	// We can confirm that by looking at the message metadata for the
	// *num delivered* field which will be 2 since this is the second time
	// it was delivered.
	md, _ := batch[0].Metadata()
	fmt.Printf("num delivered == 2? %v\n", md.NumDelivered == 2)

	// ### Max in-flight messages
	// The first limit to explore is the max in-flight messages. This
	// will limit how many un-acked in-flight messages there are. We
	// can update the consumer config after setting this value.
	// (note only certain options can be changed).
	fmt.Println("\n--- max in-flight messages (n=5) ---")

	sub = resetStreamConsumerSub(js, sub, streamName, &nats.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       5 * time.Second,
		MaxAckPending: 5,
	}, updateConfig)

	publishEvents(js, streamName, 20)

	// Now if try to fetch more than five messages, we will only be given
	// five since none are currently in-flight.
	batch, _ = sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("requested 10, got %d\n", len(batch))
	checkRedelivered(batch)

	// This limit becomes more apparent with the second fetch which would
	// timeout since we haven't acked the previous five yet.
	_, err = sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("timeout error? %v\n", err == nats.ErrTimeout)

	// Once we ack the first batch, then we can fetch some more.
	ackMsgs(batch)

	// Let's fetch the remaining 10 and ack them..
	batch, _ = sub.Fetch(5, nats.MaxWait(time.Second))
	fmt.Printf("requested 5, got %d\n", len(batch))
	checkRedelivered(batch)
	ackMsgs(batch)

	// ### Max fetch batch size
	// This one simply limits the max batch size any one fetch can receive.
	// We will also reset the max ack pending so we don't confuse which limit
	// is taking effect.
	fmt.Println("\n--- max fetch batch size (n=5) ---")

	sub = resetStreamConsumerSub(js, sub, streamName, &nats.ConsumerConfig{
		Durable:         consumerName,
		AckPolicy:       nats.AckExplicitPolicy,
		AckWait:         5 * time.Second,
		MaxRequestBatch: 5,
	}, updateConfig)

	publishEvents(js, streamName, 20)

	// Note that although multiple requests can be made (without acking),
	// each one will return at most the max request batch size.
	batch, err = sub.Fetch(10, nats.MaxWait(time.Second))
	fmt.Printf("requested 10, got %d -> %s\n", len(batch), err)

	batch, err = sub.Fetch(6, nats.MaxWait(time.Second))
	fmt.Printf("requested 6, got %d -> %s\n", len(batch), err)

	batch, err = sub.Fetch(5, nats.MaxWait(time.Second))
	fmt.Printf("requested 5, got %d\n", len(batch))

	// ### Max in-flight requests
	// The second limit we will add constrains the number of in-flight
	// pull requests with unack'ed messages across subscribers. We will
	// also revert the max ack pending to focus on this new limit.
	fmt.Println("\n--- max in-flight requests (n=2) ---")

	sub = resetStreamConsumerSub(js, sub, streamName, &nats.ConsumerConfig{
		Durable:    consumerName,
		AckPolicy:  nats.AckExplicitPolicy,
		AckWait:    5 * time.Second,
		MaxWaiting: 2,
	}, updateConfig)

	wg := &sync.WaitGroup{}
	wg.Add(3)

	// For the current subscription we have, we can make more than *max
	// waiting* fetch requests since the configuration applies across
	// subscriptions.
	go func() {
		b, err := sub.Fetch(1, nats.MaxWait(time.Second))
		fmt.Printf("sub1: fetch 1 ok? got %d or %v\n", len(b), err)
		wg.Done()
	}()

	go func() {
		b, err := sub.Fetch(1, nats.MaxWait(time.Second))
		fmt.Printf("sub1: fetch 2 ok? got %d or %v\n", len(b), err)
		wg.Done()
	}()

	go func() {
		b, err := sub.Fetch(1, nats.MaxWait(time.Second))
		fmt.Printf("sub1: fetch 3 ok? got %d or %v\n", len(b), err)
		wg.Done()
	}()

	wg.Wait()

	// ### Max fetch timeout
	// Shown above, we are explicitly setting the MaxWait option on per
	// fetch request, however this can be set and capped as part of the
	// consumer config.
	sub = resetStreamConsumerSub(js, sub, streamName, &nats.ConsumerConfig{
		Durable:           consumerName,
		AckPolicy:         nats.AckExplicitPolicy,
		AckWait:           5 * time.Second,
		MaxRequestExpires: 3 * time.Second,
	}, updateConfig)

	// All messages have been acked in the stream, so this request should
	// timeout. We will set a longer MaxWait time to demonstrate.
	t0 := time.Now()
	batch, err = sub.Fetch(10, nats.MaxWait(10*time.Second))
	fmt.Printf("expiry occured? %s in %s\n", err, time.Since(t0))

	// ### Max total bytes per fetch
	// This is not yet implememented...
}

func resetStreamConsumerSub(js nats.JetStreamContext, sub *nats.Subscription, stream string, cfg *nats.ConsumerConfig, update bool) *nats.Subscription {
	_, err := js.StreamInfo(stream)
	// No error means it exists, so do a consumer-only update.
	if err == nil && update {
		// Temporary update for clearing the current messages...
		_, err = js.UpdateConsumer(stream, &nats.ConsumerConfig{
			Durable:   cfg.Durable,
			AckPolicy: nats.AckExplicitPolicy,
			AckWait:   30 * time.Second,
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("cleaning up pending...")
		msgs, err := sub.Fetch(100)
		if err != nil && err != nats.ErrTimeout {
			log.Fatal(err)
		}
		ackMsgs(msgs)

		// Now update with the real config.
		js.UpdateConsumer(stream, cfg)
		return sub
	}

	// Remove existing consumer and stream if present to reset the example.
	js.DeleteConsumer(stream, cfg.Durable)
	js.DeleteStream(stream)

	// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/go/).
	js.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{fmt.Sprintf("%s.>", stream)},
	})

	js.AddConsumer(stream, cfg)

	sub, _ = js.PullSubscribe("", cfg.Durable, nats.Bind(stream, cfg.Durable))
	return sub
}

// Helper function to publish some events to the stream.
func publishEvents(js nats.JetStreamContext, stream string, count int) {
	for i := 0; i < count; i++ {
		subject := fmt.Sprintf("%s.%d", stream, i%5)
		js.PublishAsync(subject, nil)
	}

	<-js.PublishAsyncComplete()
	fmt.Printf("published %d messages\n", count)
}

func checkRedelivered(msgs []*nats.Msg) {
	var count int
	for _, m := range msgs {
		md, _ := m.Metadata()
		if md.NumDelivered > 1 {
			count++
		}
	}
	if count > 0 {
		fmt.Printf("%d messages were redelivered\n", count)
	}
}

func ackMsgs(msgs []*nats.Msg) {
	for _, m := range msgs {
		m.Ack()
	}
	fmt.Printf("acked %d messages\n", len(msgs))
}
