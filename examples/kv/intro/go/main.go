package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	nc, _ := nats.Connect(url)
	defer nc.Drain()

	js, _ := jetstream.New(nc)

	// JetStream API uses context for timeouts and cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ### Bucket basics
	// A key-value (KV) bucket is created by specifying a bucket name.
	kv, _ := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "profiles",
	})

	// As one would expect, the `KeyValue` interface provides the
	// standard `Put` and `Get` methods. However, unlike most KV
	// stores, a revision number of the entry is tracked.
	kv.Put(ctx, "sue.color", []byte("blue"))
	entry, _ := kv.Get(ctx, "sue.color")
	fmt.Printf("%s @ %d -> %q\n", entry.Key(), entry.Revision(), string(entry.Value()))

	kv.Put(ctx, "sue.color", []byte("green"))
	entry, _ = kv.Get(ctx, "sue.color")
	fmt.Printf("%s @ %d -> %q\n", entry.Key(), entry.Revision(), string(entry.Value()))

	// A revision number is useful when you need to enforce [optimistic
	// concurrency control][occ] on a specific key-value entry. In short,
	// if there are multiple actors attempting to put a new value for a
	// key concurrently, we want to prevent the "last writer wins" behavior
	// which is non-deterministic. To guard against this, we can use the
	// `kv.Update` method and specify the expected revision. Only if this
	// matches on the server, will the value be updated.
	// [occ]: https://en.wikipedia.org/wiki/Optimistic_concurrency_control
	_, err := kv.Update(ctx, "sue.color", []byte("red"), 1)
	fmt.Printf("expected error: %s\n", err)

	kv.Update(ctx, "sue.color", []byte("red"), 2)
	entry, _ = kv.Get(ctx, "sue.color")
	fmt.Printf("%s @ %d -> %q\n", entry.Key(), entry.Revision(), string(entry.Value()))

	// ### Stream abstraction
	// Before moving on, it is important to understand that a KV bucket is
	// light abstraction over a standard stream. This is by design since it
	// enables some powerful features which we will observe in a minute.
	//
	// **How exactly is a KV bucket modeled as a stream?**
	// When one is created, internally, a stream is created using the `KV_`
	// prefix as convention. Appropriate stream configuration are used that
	// are optimized for the KV access patterns, so you can ignore the
	// details.
	name := <-js.StreamNames(ctx).Name()
	fmt.Printf("KV stream name: %s\n", name)

	// Since it is a normal stream, we can create a consumer and
	// fetch messages.
	// If we look at the subject, we will notice that first token is a
	// special reserved prefix, the second token is the bucket name, and
	// remaining suffix is the actually key. The bucket name is inherently
	// a namespace for all keys and thus there is no concern for conflict
	// across buckets. This is different from what we need to do for a stream
	// which is to bind a set of _public_ subjects to a stream.
	cons, _ := js.CreateOrUpdateConsumer(ctx, "KV_profiles", jetstream.ConsumerConfig{
		AckPolicy: jetstream.AckNonePolicy,
	})

	msg, _ := cons.Next()
	md, _ := msg.Metadata()
	fmt.Printf("%s @ %d -> %q\n", msg.Subject(), md.Sequence.Stream, string(msg.Data()))

	// Let's put a new value for this key and see what we get from the subscription.
	kv.Put(ctx, "sue.color", []byte("yellow"))
	msg, _ = cons.Next()
	md, _ = msg.Metadata()
	fmt.Printf("%s @ %d -> %q\n", msg.Subject(), md.Sequence.Stream, string(msg.Data()))

	// Unsurprisingly, we get the new updated value as a message.
	// Since it's KV interface, we should be able to delete a key as well.
	// Does this result in a new message?
	kv.Delete(ctx, "sue.color")
	msg, _ = cons.Next()
	md, _ = msg.Metadata()
	fmt.Printf("%s @ %d -> %q\n", msg.Subject(), md.Sequence.Stream, msg.Data())

	// 🤔 That is useful to get a message that something happened to that key,
	// and that this is considered a new revision.
	// However, how do we know if the new value was set to be `nil` or the key
	// was deleted?
	// To differentiate, delete-based messages contain a header. Notice the `KV-Operation: DEL`
	// header.
	fmt.Printf("headers: %v\n", msg.Headers())

	// ### Watching for changes
	// Although one could subscribe to the stream directly, it is more convenient
	// to use a `KeyWatcher` which provides a deliberate API and types for tracking
	// changes over time. Notice that we can use a wildcard which we will come back to..
	w, _ := kv.Watch(ctx, "sue.*")
	defer w.Stop()

	// Even though we deleted the key, of course we can put a new value.
	kv.Put(ctx, "sue.color", []byte("purple"))

	// If we receive from the *updates* channel, the value is a `KeyValueEntry`
	// which exposes more KV-specific information than the raw stream message
	// shown above (so this API is recommended).
	// Since we initialized this watcher prior to setting the new color, the
	// first entry will contain the delete operation.
	kve := <-w.Updates()
	fmt.Printf("%s @ %d -> %q (op: %s)\n", kve.Key(), kve.Revision(), string(kve.Value()), kve.Operation())

	// The first value was an *initial value* which is then followed by a nil entry, so we will ignore this.
	// however, this can be useful to known when the watcher has _caught up_ with the current updates before
	// tracking the new ones.
	<-w.Updates()

	// After the watcher was initialized, we put a new color, so we will observe this change as well.
	kve = <-w.Updates()
	fmt.Printf("%s @ %d -> %q (op: %s)\n", kve.Key(), kve.Revision(), string(kve.Value()), kve.Operation())

	// To finish this short intro, since we know that keys are subjects under the covers, if we
	// put another key, we can observe the change through the watcher. One other detail to call out
	// is notice the revision for this *new* key is not `1`. It relies on the underlying stream's
	// message sequence number to indicate the _revision_. The guarantee being that it is always
	// monotonically increasing, but numbers will be shared across keys (like subjects) rather
	// than sequence numbers relative to each key.
	kv.Put(ctx, "sue.food", []byte("pizza"))

	kve = <-w.Updates()
	fmt.Printf("%s @ %d -> %q (op: %s)\n", kve.Key(), kve.Revision(), string(kve.Value()), kve.Operation())
}
