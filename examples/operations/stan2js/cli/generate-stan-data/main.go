package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/stan.go"
)

func main() {
	var (
		context  string
		cluster  string
		clientID string
	)

	flag.StringVar(&context, "context", "", "NATS context name")
	flag.StringVar(&cluster, "cluster", "", "STAN cluster name")
	flag.StringVar(&clientID, "client", "", "STAN client ID")

	flag.Parse()

	nc, err := natscontext.Connect(context)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Drain()

	// Using the client "stan2js" for demonstration.
	sc, err := stan.Connect(cluster, clientID, stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	defer sc.Close()

	// Create a few channels.
	for i := 0; i < 1000; i++ {
		err = sc.Publish("foo", []byte(fmt.Sprintf("foo: %d", i)))
		if err != nil {
			log.Fatal(err)
		}

		err = sc.Publish("bar", []byte(fmt.Sprintf("bar: %d", i)))
		if err != nil {
			log.Fatal(err)
		}

		err = sc.Publish("baz", []byte(fmt.Sprintf("baz: %d", i)))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create a few durable subs.
	var sub stan.Subscription
	seqch := make(chan uint64)

	// This function is used to create a callback for each durable
	// and ack the number of messages given by `num` and closes the
	// subscription when the last message is acked. This is to
	// emulate progress.
	newCb := func(num int, seqch chan uint64) func(m *stan.Msg) {
		i := 0
		return func(m *stan.Msg) {
			m.Ack()
			i++
			if i == num {
				sub.Close()
				seqch <- m.Sequence
				return
			}
		}
	}

	sub, err = sc.Subscribe(
		"foo",
		newCb(100, seqch),
		stan.DurableName("sub-foo"),
		stan.DeliverAllAvailable(),
		stan.SetManualAckMode(),
	)

	<-seqch

	sub, err = sc.QueueSubscribe(
		"bar",
		"q", newCb(50, seqch),
		stan.DurableName("sub-bar-q"),
		stan.DeliverAllAvailable(),
		stan.SetManualAckMode(),
	)

	<-seqch

	sub, err = sc.Subscribe(
		"bar",
		newCb(400, seqch),
		stan.DurableName("sub-bar"),
		stan.DeliverAllAvailable(),
		stan.SetManualAckMode(),
	)

	<-seqch
}
