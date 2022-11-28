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

	// Emulate a Random Number Generator service.
	nc.Subscribe("rng", func(msg *nats.Msg) {
		// Assume the message payload is a valid integer.
		n, _ := strconv.ParseInt(string(msg.Data), 10, 64)

		// Get the reply subject to stream messages to.
		reply := msg.Header.Get("x-stream-subject")

		// Respond to the initial request with a nil message indicating everything
		// is OK and the messages will be streamed. If there was a validation issue
		// or other problem, an error response could be returned to tell the client
		// to close the subscription.
		msg.Respond(nil)

		// Stream the responds to the client.
		for i := 0; i < int(n); i++ {
			r := rand.Intn(100)
			nc.Publish(reply, []byte(fmt.Sprintf("%d", r)))
		}

		// Publish nil data to indicate end-of-stream.
		nc.Publish(reply, nil)
	})

	// Generate a new random inbox subject for the server stream.
	streamSubject := nc.NewInbox()

	// Setup a subscription on that unique subject to buffer messages.
	sub, _ := nc.SubscribeSync(streamSubject)

	// Construct the request message, including the stream header and the
	// number of random numbers to generate.
	msg := nats.NewMsg("rng")
	msg.Data = []byte("10")
	msg.Header.Add("x-stream-subject", streamSubject)

	// Assume all is ok for the example..
	nc.RequestMsg(msg, time.Second)

	for {
		msg, _ := sub.NextMsg(time.Second)
		// Indicates the end of the stream.
		if len(msg.Data) == 0 {
			break
		}

		// Print the random number.
		fmt.Println(string(msg.Data))
	}
}
