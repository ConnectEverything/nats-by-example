package main

import (
	"fmt"
	"os"
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

	// In addition to vanilla publish-request, NATS supports request-reply
	// interactions as well. Under the covers, this is just an optimized
	// pair of publish-subscribe operations.
	// The _request handler_ is just a subscription that _responds_ to a message
	// sent to it. This kind of subscription is called a _service_.
	// For this example, we can use the built-in asynchronous
	// subscription in the Go SDK.
	sub, _ := nc.Subscribe("greet.*", func(msg *nats.Msg) {
		// Parse out the second token in the subject (everything after greet.)
		// and use it as part of the response message.
		name := msg.Subject[6:]
		msg.Respond([]byte("hello, " + name))
	})

	// Now we can use the built-in `Request` method to do the service request.
	// We simply pass a nil body since that is being used right now. In addition,
	// we need to specify a timeout since with a request we are _waiting_ for the
	// reply and we likely don't want to wait forever.
	rep, _ := nc.Request("greet.joe", nil, time.Second)
	fmt.Println(string(rep.Data))

	rep, _ = nc.Request("greet.sue", nil, time.Second)
	fmt.Println(string(rep.Data))

	rep, _ = nc.Request("greet.bob", nil, time.Second)
	fmt.Println(string(rep.Data))

	// What happens if the service is _unavailable_? We can simulate this by
	// unsubscribing our handler from above. Now if we make a request, we will
	// expect an error.
	sub.Unsubscribe()

	_, err := nc.Request("greet.joe", nil, time.Second)
	fmt.Println(err)
}
