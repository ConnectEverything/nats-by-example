package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
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

	// Create an async subscription that will respond to `greet` requests.
	// This will unmarshal the request and reply with a new message.
	nc.Subscribe("greet", func(msg *nats.Msg) {
		var req GreetRequest
		proto.Unmarshal(msg.Data, &req)

		rep := GreetReply{
			Text: fmt.Sprintf("hello %q!", req.Name),
		}
		data, _ := proto.Marshal(&rep)
		msg.Respond(data)
	})

	// Allocate the request type which is a generate protobuf type.
	// and Marshal it using the `proto` package.
	req := GreetRequest{
		Name: "joe",
	}
	data, _ := proto.Marshal(&req)

	// Messages are published to subjects. Although there are no subscribers,
	// this will be published successfully.
	msg, _ := nc.Request("greet", data, time.Second)

	var rep GreetReply
	proto.Unmarshal(msg.Data, &rep)

	fmt.Printf("reply: %s\n", rep.Text)
}
