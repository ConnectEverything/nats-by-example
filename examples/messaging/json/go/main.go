package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

type payload struct {
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

func main() {
	// Use the env variable if running in the container, otherwise use the default.
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Create an unauthenticated connection to NATS.
	nc, _ := nats.Connect(url)
	defer nc.Drain()

	// Create a subscription that receives two messages. One message will
	// contain a valid serialized payload and the other will not.
	sub, _ := nc.SubscribeSync("foo")
	sub.AutoUnsubscribe(2)

	// Construct a Payload value and serialize it.
	p := &payload{
		Foo: "bar",
		Bar: 27}

	p_json, _ := json.Marshal(p)

	// Publish the serialized payload.
	nc.Publish("foo", p_json)
	nc.Publish("foo", []byte("not json"))

	// Loop through the expected messages and  attempt to deserialize the payload
	// into a payload value. If deserialization into this type fails, alternate
	// handling can be performed, either discarding or attempting to deserialize in
	// a more general type (such as the empty interface):
	// var dat map[string]interface{}
	var dat payload
	for {
		msg, err := sub.NextMsg(time.Second)
		if err != nil {
			break
		}

		err = json.Unmarshal(msg.Data, &dat)
		if err != nil {
			fmt.Printf("received invalid JSON payload: %s\n", msg.Data)
		} else {
			fmt.Printf("received valid JSON payload: %+v\n", dat)
		}
	}

}
