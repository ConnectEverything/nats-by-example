package main

import (
	"context"
	"log"
	"math"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
)

func main() {
	natsURL := os.Getenv("NATS_URL")

	nc, _ := nats.Connect(natsURL)
	defer nc.Drain()

	js, _ := nc.JetStream()

	// Create an [interest-based stream][interest] which will hold
	// a message only until all bound consumers have ack'ed the message.
	// [interest]: /examples/jetstream/interest-stream/go
	js.AddStream(&nats.StreamConfig{
		Name:      "TEST",
		Subjects:  []string{"test.*"},
		Retention: nats.InterestPolicy,
	})

	// Create pull consumer that will track state for the subscribers
	// that actually process the messages.
	js.AddConsumer("TEST", &nats.ConsumerConfig{
		Name:      "PROCESSOR",
		AckPolicy: nats.AckExplicitPolicy,
	})

	// Create pull consumer that will monitor the stream and auto-provision
	// subscribers bound to the PROCESSOR consumer to actually do the work.
	js.AddConsumer("TEST", &nats.ConsumerConfig{
		Name:        "PROVISIONER",
		AckPolicy:   nats.AckAllPolicy,
		HeadersOnly: true,
	})

	// Setup a base context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize a basic goroutine provisioner that implements `Provisioner`.
	provisioner := newGoroutineProvision(js, "TEST", "PROCESSOR")

	// Start the provisioner which actually monitors the stream and invokes
	// the provisioner methods.
	sub, _ := StartProvisioner(ctx, js, provisioner, "TEST", "PROVISIONER")
	defer sub.Unsubscribe()

	// Send a batch of requests which will scale up
	log.Print("starting batch 1...")
	t0 := time.Now()
	for i := 0; i < 10_000; i++ {
		id := nuid.Next()
		for {
			_, err := js.Request("test.foo", nil, time.Second, nats.MsgId(id))
			if err == nil {
				break
			} else {
				log.Printf("batch 1: %d: %s, retrying...", i, err)
			}
		}
	}
	log.Printf("first batch done in %s", time.Since(t0))

	log.Print("sleeping 5 seconds..")
	time.Sleep(3 * time.Second)

	log.Print("starting batch 2...")
	t0 = time.Now()
	for i := 0; i < 10_000; i++ {
		id := nuid.Next()
		for {
			_, err := js.Request("test.foo", nil, time.Second, nats.MsgId(id))
			if err == nil {
				break
			} else {
				log.Printf("batch 2: %d: %s, retrying...", i, err)
			}
		}
	}
	log.Printf("second batch done in %s", time.Since(t0))

	cancel()
	time.Sleep(time.Second)
}

func StartProvisioner(ctx context.Context, js nats.JetStreamContext, p Provisioner, stream, consumer string) (*nats.Subscription, error) {
	// Bind a subscriber for PROVISIONER consumer.
	sub, err := js.PullSubscribe("", "", nats.Bind(stream, consumer))
	if err != nil {
		return nil, err
	}

	go startProvisioner(ctx, sub, p)

	return sub, nil
}

func startProvisioner(ctx context.Context, sub *nats.Subscription, p Provisioner) {
	var (
		count         int
		ratePerSec    float64
		lastReceive   time.Time
		lastScaleTime time.Time
	)

	startTime := time.Now()
	instances := make(map[string]struct{})

	for {
		fctx, cancel := context.WithTimeout(ctx, time.Second)

		// Timeout getting the next message after a second.. this will ensure
		// when there are no new messages, the scaling check will run to
		// scale down.
		msgs, err := sub.Fetch(10, nats.Context(fctx))
		cancel()

		stopping := false
		switch err {
		case nil:
		case nats.ErrTimeout, context.DeadlineExceeded:
			log.Print("provisioner: no new messages")
		default:
			stopping = true
			log.Printf("provisioner: stopping: %s", err)
		}

		now := time.Now()

		// Update stats when messages are present.
		if l := len(msgs); l > 0 {
			msgs[l-1].Ack()

			count += l
			lastReceive = now
			dur := lastReceive.Sub(startTime)
			ratePerSec = float64(count) / (float64(dur) / float64(time.Second))
			if len(instances) == 0 {
				log.Printf("provisioner: new message observed")
			}
		}

		if stopping || now.Sub(lastScaleTime) >= time.Second {
			lastScaleTime = now
			// Determine desired scale of instances.
			currentNum := len(instances)
			newNum := 0
			if !stopping {
				newNum = p.NumInstances(lastReceive, currentNum, ratePerSec)
			}
			scaleDiff := newNum - currentNum
			if scaleDiff != 0 {
				log.Printf("provisioner: scaling: %d -> %d", currentNum, newNum)
			}

			if scaleDiff < 0 {
				var n int
				for id := range instances {
					if n == -scaleDiff {
						break
					}
					n++
					err := p.StopInstance(ctx, id)
					if err != nil {
						log.Printf("provisioner: instance %s error: %s", id, err)
					} else {
						log.Printf("provisioner: instance %s stopped", id)
						delete(instances, id)
					}
				}
			}

			if scaleDiff > 0 {
				for i := 0; i < scaleDiff; i++ {
					id, err := p.StartInstance(ctx)
					if err != nil {
						log.Printf("provisioner: instance %s error: %s", id, err)
						break
					}
					instances[id] = struct{}{}
					log.Printf("provisioner: instance %s started", id)
				}
			}

			if stopping {
				return
			}
		}
	}
}

type Provisioner interface {
	// NumInstances returns the number of instances that should be provisioned
	// given the last message receive time, the current number, and the current
	// message rate per second.
	NumInstances(lastReceive time.Time, num int, ratePerSec float64) int

	// StartInstance starts a new instance and returns and ID representing the
	// instance.
	StartInstance(ctx context.Context) (string, error)

	// StopInstance stops an instance given an ID.
	StopInstance(ctx context.Context, id string) error
}

func newGoroutineProvision(js nats.JetStreamContext, stream, consumer string) *goroutineProvisioner {
	return &goroutineProvisioner{
		js:                js,
		inactiveThreshold: 2 * time.Second,
		fetchSize:         5,
		maxRatePerHandler: 500,
		streamName:        stream,
		consumerName:      consumer,
		handlers:          make(map[string]context.CancelFunc),
	}
}

type goroutineProvisioner struct {
	js                nats.JetStreamContext
	inactiveThreshold time.Duration
	maxRatePerHandler int
	fetchSize         int
	streamName        string
	consumerName      string
	handlers          map[string]context.CancelFunc
}

// Function to calculate the appropriate number of handlers
// given the message rate.
func (p *goroutineProvisioner) NumInstances(lastReceive time.Time, current int, ratePerSec float64) int {
	// Detect inactivity, gradually scale down.
	if time.Since(lastReceive) >= p.inactiveThreshold {
		return current - 1
	}

	// Start with 1
	if current == 0 {
		return 1
	}

	// Scale up one at a time until the max is reached based on handling 100
	current++
	max := int(math.Ceil(ratePerSec / float64(p.maxRatePerHandler)))

	if current > max {
		current = max
	}

	return current
}

func (d *goroutineProvisioner) startInstance(ctx context.Context, id string) {
	sub, _ := d.js.PullSubscribe("", "", nats.Bind(d.streamName, d.consumerName))
	defer sub.Drain()

	count := 0

	for {
		fctx, cancel := context.WithTimeout(ctx, time.Second)
		msgs, err := sub.Fetch(d.fetchSize, nats.Context(fctx))
		cancel()

		switch err {
		// Received messages, or no new messages so just continue.
		case nil, context.DeadlineExceeded:
			// Clean exit.
		case context.Canceled:
			log.Printf("instance %s: processed %d messages", id, count)
			return
		default:
			log.Printf("instance %s: error: %s", id, err)
			return
		}

		// Loop and do work..
		for _, msg := range msgs {
			msg.Respond(nil)
			msg.Ack()
			count++
		}
	}
}

func (d *goroutineProvisioner) StartInstance(ctx context.Context) (string, error) {
	id := nuid.Next()
	hctx, cancel := context.WithCancel(ctx)
	d.handlers[id] = cancel
	go d.startInstance(hctx, id)
	return id, nil
}

func (d *goroutineProvisioner) StopInstance(ctx context.Context, id string) error {
	cancel, ok := d.handlers[id]
	if ok {
		cancel()
		delete(d.handlers, id)
	}
	return nil
}
