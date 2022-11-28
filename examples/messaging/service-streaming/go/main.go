package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	// Use the env varibale if running in the container, otherwise use the default.
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Create an unauthenticated connection to NATS.
	nc, _ := nats.Connect(url)
	defer nc.Drain()

	// Setup a simple random number generator (RNG) service that
	// streams numbers. Using a queue group ensures that only one
	// of the members will receive the request to process.
	nc.QueueSubscribe("rng", "rng", func(msg *nats.Msg) {
		// Do request validation, etc. If there are any issues, a response
		// can be sent with any error prior to the stream starting.
		n, _ := strconv.ParseInt(string(msg.Data), 10, 64)

		// Extract the unique subject to stream messages on.
		subject := msg.Header.Get("x-stream-subject")

		// Respond to the initial request with any errors. Otherwise an
		// empty reply indicates the stream will start.
		msg.Respond(nil)

		// Stream the numbers to the client. A publish is used here,
		// however, if ack'ing is desired, a request could be used
		// per message.
		for i := 0; i < int(n); i++ {
			r := rand.Intn(100)
			nc.Publish(subject, []byte(fmt.Sprintf("%d", r)))
		}

		// Publish empty data to indicate end-of-stream.
		nc.Publish(subject, nil)
	})

	// Generate a unique inbox subject for the stream of messages
	// and subscribe to it.
	inbox := nc.NewInbox()
	sub, _ := nc.SubscribeSync(inbox)

	// Prepare the message to initiate the interaction. The inbox
	// subject is included in the header for the service to extract
	// and publish to.
	msg := nats.NewMsg("rng")
	msg.Header.Set("x-stream-subject", inbox)
	msg.Data = []byte("10")

	// Send the request to initiate the interaction.
	nc.RequestMsg(msg, time.Second)

	// Loop to receive all messages over the stream.
	for {
		msg, err := sub.NextMsg(time.Second)
		// Handle error.
		if err != nil {
			sub.Unsubscribe()
			// handle error
			break
		}

		// Indicates the end of the stream.
		if len(msg.Data) == 0 {
			sub.Unsubscribe()
			break
		}

		// Print the random number.
		fmt.Println(string(msg.Data))
	}
}
