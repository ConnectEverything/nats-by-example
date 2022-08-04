package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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

	// Create an unauthenticated connection to NATS and defer the drain
	// to gracefully handle buffered messages and clean up resources.
	nc, _ := nats.Connect(url)
	defer nc.Drain()

	// Access `JetStreamContext` which provides methods to create
	// streams and consumers as well as convenience methods for publishing
	// to streams and implicitly creating consumers through `*Subscribe*`
	// methods (which will be discussed in examples focused on consumers).
	js, _ := nc.JetStream()

	// Given a stream that is being written to by a backend  control plane
	// service needing to communicate out to a fleet of devices, we can
	// enable and leverage the republish option on a stream. This takes a
	// subject mapping from the source (subjects bound to the stream) and
	// some destination subject.
	//
	// For this example, assume a source subject
	// like `config.<device>.<property...>`. We are then republishing messages
	// to `<device>.config.<property...>` by re-arranging the wildcard tokens.
	// *Note: the Go client prior to v1.16.0 used an experimental API for
	// this feature and changed the name of the type for `RePublish` from
	// `SubjectMapping`.*
	bucketName := "CONFIG"

	kv, _ := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:  bucketName,
		History: 1,
		RePublish: &nats.RePublish{
			Source:      ">",
			Destination: "repub.>",
		},
	})

	// For the purpose of this example, let's assume we have a known set of
	// configuration properties. We are also going to assume that we
	// only care about the latest value for each property. When a device
	// receives a value, it will update it's state accordingly.
	props := []string{
		"driving",
		"temperature",
		"radio",
	}

	// Next we will create a subscriptions to a particular device.
	// In practice, there can be N devices each with their own subscription
	// using their own connection. This would be reuqire when combining
	// with user permissions. Each device can have user credentials tied
	// a set of permissions authorized to only subscribe to its subset
	// of messages. This provides a lightweight and secure fan-out option
	// in lieu of consumers.
	sub, _ := nc.SubscribeSync("repub.$KV.CONFIG.device-1.*")

	// Let's publish a message to the stream and then receive it
	// on the subscription.
	kv.Put("device-1.driving", []byte(`true`))

	msg, _ := sub.NextMsg(time.Second)
	fmt.Printf("received message on %q\n", msg.Subject)

	// Every republished message includes an augmented set of headers
	// which allows for tracking and correlating this message with respect
	// to the original stream.
	fmt.Printf(`republish headers:
- Nats-Subject: %s
- Nats-Stream: %s
- Nats-Sequence: %s
- Nats-Last-Sequence: %s
`,
		msg.Header.Get("Nats-Subject"),
		msg.Header.Get("Nats-Stream"),
		msg.Header.Get("Nats-Sequence"),
		msg.Header.Get("Nats-Last-Sequence"),
	)

	// Assuming we want to keep track of these properties as they come in,
	// we can use a simple config map that is *sequence* aware. Meaning, it
	// keeps keeps track of the sequence to ensure only the *latest* value
	// per property is set in the config.
	config := NewConfig()

	// Let's parse out the property key, sequence value, and value from
	// from the message and set it into the config and print it.
	prop, seq, val := parseConfigEntry(msg)
	config.Set(prop, seq, val)
	fmt.Printf("config: %s\n", config)

	// And another...
	kv.Put("device-1.temperature", []byte(`80`))

	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received message on %q\n", msg.Subject)

	prop, seq, val = parseConfigEntry(msg)
	config.Set(prop, seq, val)
	fmt.Printf("config: %s\n", config)

	// What happens if the device temporarily gets disconnected? How can
	// we catch up? Since the messages are being republished to a subject
	// not bound to the stream, the delivery guarantee is at-most-once.
	// This is the trade-off with consumers (which provide at-least-once),
	// but this republish capability can, currently, scale to orders of
	// magnitude more devices.
	// Let's unsubscribe and observe what happens.
	sub.Unsubscribe()
	fmt.Println("subscription closed")

	kv.Put("device-1.temperature", []byte(`72`))
	kv.Put("device-1.temperature", []byte(`76`))

	// To recover from this situation, we can introduce an initialization
	// step after we setup the subscription. Let's re-subscribe to start
	// buffering new messages.
	sub, _ = nc.SubscribeSync("repub.$KV.CONFIG.device-1.*")
	fmt.Println("re-subscribed for new messages")

	// This API is slightly lower-level and returns a `RawStreamMsg`.
	// We can still extract the same information we need to update the
	// config struct.
	fmt.Println("direct latest props")
	for _, prop := range props {
		// Original subject?
		subject := fmt.Sprintf("device-1.%s", prop)
		entry, _ := kv.Get(subject)

		// Just ignore if no message is available.
		if entry == nil {
			continue
		}

		seq := entry.Revision()
		val := string(entry.Value())

		fmt.Printf("catching up prop: %q\n", prop)
		config.Set(prop, seq, val)
		fmt.Printf("config: %s\n", config)
	}

	// If we publish a new message to the stream, we can receive it on
	// the subscription and continue on.
	kv.Put("device-1.radio", []byte(`102.9 FM`))

	// Receive the message and set in the config.
	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received message on %q\n", msg.Subject)

	prop, seq, val = parseConfigEntry(msg)
	config.Set(prop, seq, val)
	fmt.Printf("config: %s\n", config)

	sub.Unsubscribe()
}

type SeqValue struct {
	Seq   uint64
	Value string
}

type Config struct {
	props map[string]*SeqValue
	mux   sync.RWMutex
}

func (c *Config) String() string {
	c.mux.RLock()
	defer c.mux.RUnlock()
	m := make(map[string]string)
	for k, v := range c.props {
		m[k] = v.Value
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func (c *Config) Get(key string) string {
	c.mux.RLock()
	defer c.mux.RUnlock()

	sv, ok := c.props[key]
	if !ok {
		return ""
	}
	return sv.Value
}

func (c *Config) Set(key string, seq uint64, val string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	sv, ok := c.props[key]
	if !ok {
		c.props[key] = &SeqValue{
			Seq:   seq,
			Value: val,
		}
		return
	}

	// Update if more recent.
	if seq > sv.Seq {
		sv.Seq = seq
		sv.Value = val
	}
}

func NewConfig() *Config {
	return &Config{
		props: make(map[string]*SeqValue),
	}
}

func parseConfigEntry(msg *nats.Msg) (string, uint64, string) {
	// Parse out the property key and string-encode the value
	// (for demonstration) and set the value on the config.
	idx := strings.LastIndexByte(msg.Subject, '.')
	prop := msg.Subject[idx+1:]
	val := string(msg.Data)

	// Access metadata to get the sequence number. This cannot rely on
	// the `msg.Metadata` since this is a copy of the message from
	// the stream.
	seq, _ := strconv.ParseUint(msg.Header.Get("Nats-Sequence"), 10, 64)

	return prop, seq, val
}
