package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, _ := nats.Connect(natsURL)

	js, _ := nc.JetStream()

	js.AddStream(&nats.StreamConfig{
		Name:      "EVENTS",
		Subjects:  []string{"events"},
		Storage:   nats.MemoryStorage,
		Retention: nats.WorkQueuePolicy,
	})

	js.AddConsumer("EVENTS", &nats.ConsumerConfig{
		Durable:   "PROCESSOR",
		AckPolicy: nats.AckExplicitPolicy,
	})

	/* Push...
	js.AddConsumer("EVENTS", &nats.ConsumerConfig{
		Durable:        "PROCESSOR",
		AckPolicy:      nats.AckExplicitPolicy,
		DeliverSubject: "processor.events",
		DeliverGroup:   "processor",
	})
	*/

	ctx := context.Background()

	// Context to stop publishers after an explicit amount of time.
	pctx, pcancel := context.WithTimeout(ctx, 2*time.Second)
	defer pcancel()
	startPublishers(pctx, nc, 250000, "events", 200)

	// Wait for publishers to be done.
	<-pctx.Done()

	sctx, scancel := context.WithCancel(ctx)
	startPullSubscribers(sctx, nc, 5, nats.Bind("EVENTS", "PROCESSOR"))

	for {
		// Get the stream and consumer info to compare published and delivered count.
		sinfo, _ := js.StreamInfo("EVENTS")
		cinfo, _ := js.ConsumerInfo("EVENTS", "PROCESSOR")

		if cinfo.Delivered.Stream == sinfo.State.LastSeq {
			fmt.Printf("%d messages processed\n", cinfo.Delivered.Stream)
			scancel()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	<-sctx.Done()
}

const (
	maxPubPerSecond = 50000
	asyncBatchSize  = 10000
)

func startPublishers(ctx context.Context, nc *nats.Conn, rate int, subject string, size int) {
	numPublishers := rate / maxPubPerSecond
	batchesPerSecond := maxPubPerSecond / asyncBatchSize
	tickerDuration := time.Second / time.Duration(batchesPerSecond)

	fmt.Printf("spawning %d publishers each with %d batches/s of %d msgs to achieve %d msgs/s, ticker duration is %s\n", numPublishers, batchesPerSecond, asyncBatchSize, rate, tickerDuration)

	for i := 0; i < numPublishers; i++ {
		nx, _ := nats.Connect(nc.ConnectedUrl())
		js, _ := nx.JetStream()

		go spawnPublisher(ctx, js, subject, size, tickerDuration)
		// Jitter..
		time.Sleep(50 * time.Millisecond)
	}
}

func spawnPublisher(ctx context.Context, js nats.JetStreamContext, subject string, size int, d time.Duration) {
	data := make([]byte, size)

	t := time.NewTicker(d)
	defer t.Stop()

	c := 0
	t0 := time.Now()
	for {
		select {
		case <-t.C:
			c += asyncBatchSize
			for i := 0; i < asyncBatchSize; i++ {
				js.PublishAsync(subject, data)
			}
			<-js.PublishAsyncComplete()
		case <-ctx.Done():
			d := time.Since(t0)
			r := float64(c) / float64(d) * float64(time.Second)
			fmt.Printf("published %d messages in %s (%f msgs/s)\n", c, d, r)
			return
		}
	}
}

func startPullSubscribers(ctx context.Context, nc *nats.Conn, count int, bindOpt nats.SubOpt) {
	for i := 0; i < count; i++ {
		nx, _ := nats.Connect(nc.ConnectedUrl())
		js, _ := nx.JetStream()

		go spawnPullSubscriber(ctx, js, bindOpt)
	}
}

func spawnPullSubscriber(ctx context.Context, js nats.JetStreamContext, bindOpt nats.SubOpt) {
	sub, _ := js.PullSubscribe("", "", bindOpt)
	defer sub.Drain()

	i := 0
	for {
		msgs, err := sub.Fetch(1000)
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			log.Print(err)
			return
		}
		i++
		// Sample fetches...
		if i%100 == 0 {
			fmt.Printf("received %d msgs\n", len(msgs))
		}

		for _, msg := range msgs {
			// Do actual work..
			msg.Ack()
		}
	}
}

func startPushSubscribers(ctx context.Context, nc *nats.Conn, count int, bindOpt nats.SubOpt) {
	for i := 0; i < count; i++ {
		nx, _ := nats.Connect(nc.ConnectedUrl())
		js, _ := nx.JetStream()

		go spawnPushSubscriber(ctx, js, bindOpt)
	}
}

func spawnPushSubscriber(ctx context.Context, js nats.JetStreamContext, bindOpt nats.SubOpt) {
	sub, _ := js.QueueSubscribeSync("", "processor", bindOpt)
	defer sub.Drain()

	t0 := time.Now()
	i := 0
	for {
		msg, err := sub.NextMsg(time.Second)
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			log.Print(err)
			return
		}
		i++
		if i%50000 == 0 {
			fmt.Printf("%f msgs/s\n", float64(i)/float64(time.Since(t0))*float64(time.Second))
		}
		// Do actual work..
		msg.Ack()
	}
}
