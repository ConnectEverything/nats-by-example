package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")

	nc, _ := nats.Connect(natsURL)
	defer nc.Drain()

	js, _ := nc.JetStream()

	// Create a [work-queue][wq] stream that will act as a buffer for requests.
	js.AddStream(&nats.StreamConfig{
		Name:      "REQUESTS",
		Subjects:  []string{"requests.*"},
		Retention: nats.WorkQueuePolicy,
	})

	// Create an ephemeral consumer + subscription responsible for replying.
	sub, _ := js.Subscribe("requests.*", func(msg *nats.Msg) {
		var r string
		switch msg.Subject {
		case "requests.order-sandwich":
			r = "🥪"
		case "requests.order-bagel":
			r = "🥯"
		case "requests.order-flatbread":
			r = "🥙"
		default:
			return
		}
		msg.Respond([]byte(r))
	})
	defer sub.Drain()

	// Send some requests.
	rep, _ := js.Request("requests.order-sandwich", nil, time.Second)
	fmt.Println(string(rep.Data))

	rep, _ = js.Request("requests.order-flatbread", nil, time.Second)
	fmt.Println(string(rep.Data))

	// If a request cannot be fulfilled, the message is terminated.
	_, err := js.Request("requests.order-drink", nil, time.Second)
	fmt.Printf("timeout? %v\n", err == nats.ErrTimeout)

	info, _ := js.StreamInfo("REQUESTS")
	fmt.Printf("%d remaining in the stream\n", info.State.Msgs)
}
