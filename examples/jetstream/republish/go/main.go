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

	// Create a connection to NATS and defer the drain
	// to gracefully handle buffered messages and clean up resources.
	nc, _ := nats.Connect(url)
	defer nc.Drain()

	// Access `JetStreamContext` for convenient methods to interact
	// with JetStream constructs likes streams, consumers, and key-value.
	js, _ := nc.JetStream()

	// Given a command and control service that is writing out configuration
	// for a fleet of devices, we can leverage the key-value interface
	// overlaying a stream. With a history of 1, only the most recent
	// value per key will be retained.
	// The `RePublish` feature will take every key-value (modeled as a message)
	// and re-publish it to a destination subject. Clients can then subscribe
	// to this subject and receive all new updates while connected.
	kv, _ := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:  "CONFIG",
		History: 1,
		RePublish: &nats.RePublish{
			Source:      ">",
			Destination: "repub.>",
		},
	})

	// For the purpose of this example, let's assume we have a known set of
	// configuration properties. When a device receives a value, it will
	// update it's state accordingly.
	props := []string{
		"driving",
		"temperature",
		"radio",
	}

	// Next we will create a subscriptions to a particular device.
	// In practice, there can be N devices each with their own subscription
	// using their own connection. This would be required when combining
	// with user permissions. Each device can have user credentials tied
	// a set of permissions authorized to only subscribe to its subset
	// of messages. This provides a lightweight and secure fan-out option
	// in lieu of consumers.
	sub, _ := nc.SubscribeSync("repub.$KV.CONFIG.device-1.*")

	// Let's write an entry to the KV and observe it being received it
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
	// keeps track of the sequence to ensure only the *latest* value
	// per property is set in the config in case we receive different
	// versions concurrently.
	config := NewConfig()

	// Let's parse out the property key, sequence value, and value from
	// from the message and set it into the config.
	prop, seq, val := parseConfigEntry(msg)
	config.Set(prop, seq, val)
	fmt.Printf("config: %s\n", config)

	// Adding another property, we will observe the same flow building
	// up the config.
	kv.Put("device-1.temperature", []byte(`80`))

	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received message on %q\n", msg.Subject)

	prop, seq, val = parseConfigEntry(msg)
	config.Set(prop, seq, val)
	fmt.Printf("config: %s\n", config)

	// What happens if the device temporarily gets disconnected? How can
	// we catch up? Since the messages are being republished to a subject
	// separate from the KV, the delivery guarantee is at-most-once.
	// This is the trade-off with consumers (which provide at-least-once),
	// but this republish capability can, currently, scale to orders of
	// magnitude more devices.
	// Let's unsubscribe and observe what happens.
	sub.Unsubscribe()
	fmt.Println("subscription closed")

	kv.Put("device-1.temperature", []byte(`72`))
	kv.Put("device-1.temperature", []byte(`76`))
	fmt.Println("put 2 more kv entries...")

	// Let's re-subscribe and see if receive the last two messages?
	sub, _ = nc.SubscribeSync("repub.$KV.CONFIG.device-1.*")
	fmt.Println("re-subscribed for new messages")
	_, err := sub.NextMsg(time.Second)
	fmt.Printf("received a timeout? %v\n", err == nats.ErrTimeout)

	// As expected, we got a timeout since those previous updates would have
	// been dropped after being re-published (at-most-once guarantee). So this
	// means we need to request the KV entry for each prop for this device. It
	// doesn't yet exist, we can ignore it.
	fmt.Println("getting latest props")
	for _, prop := range props {
		subject := fmt.Sprintf("device-1.%s", prop)
		entry, _ := kv.Get(subject)
		if entry == nil {
			continue
		}

		fmt.Printf("catching up prop: %q\n", prop)
		seq := entry.Revision()
		val := string(entry.Value())
		config.Set(prop, seq, val)

		fmt.Printf("config: %s\n", config)
	}

	// If we publish a new message to the stream, we can receive it on
	// the subscription and continue on.
	kv.Put("device-1.radio", []byte(`102.9 FM`))

	msg, _ = sub.NextMsg(time.Second)
	fmt.Printf("received message on %q\n", msg.Subject)

	prop, seq, val = parseConfigEntry(msg)
	config.Set(prop, seq, val)
	fmt.Printf("config: %s\n", config)
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
